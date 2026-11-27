# MQTT: Go backend ve ESP32 gateway

D3 ve Go servisi aynı Mosquitto broker’a bağlanan iki MQTT istemcisidir. D1 ve D2 broker’a bağlanmaz. MQTT 3.1.1, UTF-8 JSON, QoS 1 ve `cleanSession=true` kullanılır. Backend için Eclipse Paho Go uygundur; bu klasör belirli bir istemci kütüphanesine bağlı değildir.

## Topic sözleşmesi

| Topic | Yön | kind | Retained |
|---|---|---|---|
| `irrigation/v1/gw-01/telemetry` | D3 → Go | `telemetry` | hayır |
| `irrigation/v1/gw-01/heartbeat` | D3 → Go | `heartbeat` | hayır |
| `irrigation/v1/gw-01/acks` | D3 → Go | `ack` | hayır |
| `irrigation/v1/gw-01/commands/node-01` | Go → D3 | `start`, `permit`, `stop` | hayır |
| `irrigation/v1/gw-01/status` | D3 / LWT → Go | `status` | evet |

Hepsinde QoS 1 kullanılır. Abone, topic ile `kind`, gateway ve node kimliğinin eşleştiğini kontrol eder. Şemaya uyan yanlış topic’teki mesaj da reddedilir. Başlatma yetkisi yalnız backend kullanıcısına aittir. [mosquitto.acl.example](mosquitto.acl.example) tek gateway için örnektir; broker’da ayrıca kimlik doğrulama ve uygun listener/TLS ayarı gerekir.

QoS 1 tekrarlı mesaj üretebilir. MQTT PUBACK, RF teslimi veya pompanın çalışması anlamına gelmez. D1’in `ack` mesajı uygulama sonucunu bildirir; `applied` yalnız sürücü çıkışının açıldığı anlamındadır, su akışı ölçümü değildir.

## Zarf ve kimlikler

Her MQTT mesajı `schema_version`, `kind`, `gateway_id`, `gateway_boot_id`, `gateway_session_id` ve `message_id` taşır.

- `gateway_boot_id`: D3 her yeniden açıldığında değişir.
- `gateway_session_id`: D3 her MQTT bağlantı girişiminde yeni, sıfır olmayan rastgele kimlik üretir. CONNECT/LWT ve o bağlantının bütün yayınlarında kullanır. Gelecek komutlar bu kimliği hedefler.
- `message_id`: yayıncının mesaj kimliği. Aynı mesajı tekrar gönderirken korunur. Global zaman sırası vermez.
- `boot_id`: RF mesajını oluşturan ESP’nin açılışı. Telemetride D1’e, komutta D3’e aittir; gateway boot alanıyla karıştırılmaz.
- `command_id`: tek anlamsal komut. İçeriği sabittir; aynı kimlikle değiştirilmiş süre veya nonce reddedilir.
- `run_id`: tek sulama çalışması. START, PERMIT ve STOP farklı komut kimlikleriyle aynı çalışmayı hedefler.
- `backend_epoch`: Go süreci her başladığında değişir. Teşhis ve uzlaştırma içindir; kriptografik yetki veya tek başına “daha yeni” kanıtı değildir.

İstemci kimlikleri farklıdır: `gateway-gw-01` ve `backend-irrigation`. Aynı istemci kimliğiyle iki süreç çalıştırılmaz. Go broker bağlantısı kesilince yeniden abone olur. Tek yetkili backend örneği varsayılır; yüksek erişilebilirlik/leader seçimi kapsam dışıdır.

## Telemetri, heartbeat ve ACK

Bu üç zarf `node_id`, `historical`, `gateway_queue_age_ms`, `last_hop_rssi_dbm` ve `frame` alanlarını taşır. RF gözlemi D3’e ulaştığında `origin=1`, `destination=3`, `hops=2`, `ttl=0` olmalıdır. D3 yalnız doğrulanmış RF gözlemini yayımlar.

Telemetri 5 saniyede bir; heartbeat 1 saniyede bir gönderilir. Heartbeat pompa çıkışını, fault kodunu, kalibrasyon sürümünü ve D1’in ürettiği `arm_nonce` değerini taşır. Backend kayıt zamanı UTC’dir. D1 `uptime_ms` alanı UTC değildir.

Gateway çevrimdışıyken en fazla 32 telemetri örneğini RAM’de tutabilir. Bağlantı gelince bunlar `historical=true` ile aktarılır. En eski düşürülen örnekler sayaçla izlenir. Eski heartbeat/ACK canlı mesaj olarak yeniden yayımlanmaz; ACK kaybı taze heartbeat ve run kimliğiyle uzlaştırılır.

Canlı örnek için muhafazakâr yaş hesabı: D1 `sample_age_ms` + gateway kuyruk yaşı + 2.000 ms RF yolu bütçesi + 1.000 ms MQTT teslim bütçesi. Son iki sayı ölçülmüş garanti değildir. Gateway canlı yayınları süreli kuyrukta tutar; broker/socket takılması veya bütçenin belirsizleşmesi durumunda bağlantıyı yeniler ve bekleyen telemetriyi geçmiş olarak işler. Yeniden bağlantıda taze heartbeat/telemetri gelmeden AUTO başlamaz. 30 saniyeyi aşan veya tarihi belirsiz ölçüm yeni START üretemez. Nihai tazelik koruması D1’in yerel nonce süresidir.

## START → PERMIT → STOP

1. Go fiziksel bölgeyi, sensörü, kontrol modunu, cooldown’u ve tek aktif run koşulunu doğrular. Run ve komut kaydını kalıcı olarak oluşturur.
2. Son taze heartbeat’teki D1 boot/nonce ve D3 gateway boot/session kimlikleriyle START yayımlar. `issued_at` ile `expires_at` arası 0’dan büyük, en çok 5.000 ms’dir; karşılaştırma UTC anları üzerinden yapılır.
3. D3 topic, şema, gateway hedefi ve zamanı kontrol eder. UTC saati güvenilir değilse START/PERMIT kabul etmez. Yerel kuyruğa giriş zamanını monoton saatle izler; bir saniyeyi aşan kontrol paketini iletmez.
4. D3 yeni RF seq verir, command/run/target boot/payload alanlarını korur. D1 doğruladıktan sonra çıkışı açıp ACK gönderir. Go, D1 onayına kadar yalnız `pending` gösterir.
5. Go, yalnız onaylanmış aktif run için her yeni heartbeat nonce’una en fazla bir PERMIT üretir. Her PERMIT yeni `command_id` alır. D3 kendiliğinden PERMIT üretmez.
6. D1’in izin sonu, nonce’un D1’deki oluşturulma zamanı + 3.000 ms’dir. Teslim anı + 3.000 ms değildir. Tekrarlanan nonce veya komut izin süresini uzatmaz. İzin bitince çıkış yerelde kapanır.
7. İlk START anından hesaplanan istenen süre ve 25.000 ms hard cap değişmez. PERMIT bu süreleri uzatamaz ve kapalı pompayı açamaz.
8. STOP aynı run ve hedef boot için çıkışı kapatır. Çıkış kapalı olsa da run terminal olarak işaretlenir; arkasından gecikmiş START gelirse çalıştırılmaz.

D3 aynı kimlikle farklı komut içeriğini reddeder. Geçici yeniden deneme, aynı anlamsal komutu ve aynı kimlikleri korur; yalnız RF seq yenilenebilir. Süresi dolmuş START yeni kimlikle otomatik tekrarlanmaz. Başlatma sonucu belirsizse yeni run açılmaz.

## STOP, zaman aşımı ve yeniden bağlanma

START/PERMIT için süresi dolmuş veya eski gateway oturumuna ait mesaj reddedilir. STOP, kimliği doğrulanmış backend’den ve doğru D1 boot/run hedefiyle geliyorsa kapanmayı kolaylaştırmak için saat/dispatch süresi geçse de işlenebilir. STOP yanlış run’a aitse aktif başka run’ı kapatmaz. Genel acil kapatma yerine geçmez; fiziksel güç kesme erişilebilir olmalıdır.

D3, gelen retained START/PERMIT mesajını işlemez. Backend bunları retained yayımlamaz; eski broker kurulumu temizlenirken varsa retained kontrol mesajları silinir. STOP da retained yayımlanmaz.

Gateway `online` yayınını retained gönderir, LWT aynı boot/session için `offline` olur. LWT gecikmeli gelebilir; `online` bilgisi D1’in erişilebilirliğini kanıtlamaz. 5 saniye heartbeat yoksa backend düğümü çevrimdışı sayar. Farklı/eski gateway oturumundan gelen durum mesajı mevcut oturumu geri alamaz; geçiş taze heartbeat ve doğrulanmış yeni bağlantıyla kurulur.

Go yeniden başladığında eski pending START’ı veya PERMIT’i tekrar yayımlamaz. Önce yeni heartbeat alır. Aktif/belirsiz run varsa STOP yollar, çıkışın kapalı olduğunu ACK veya aynı D1 boot/run içeren taze heartbeat ile doğrular. Gerekirse izin süresinin dolmasını bekler. D1 yeniden başladıysa eski run interrupted olur; yeni boot için yerel kilit uygulanır. Eski boot’lar mevcut oturumda yeniden aktif ilan edilmez.

`accepted` komutun alındığını, `applied` çıkışın açıldığını, `stopped` STOP ile kapandığını, `completed` normal süre/hedefle bittiğini, `rejected` uygulanmadığını, `fault` hata nedeniyle kapandığını belirtir. MQTT teslimi bu statülerden hiçbirine otomatik çevrilmez.

## Resmî başvuru kaynakları

- [MQTT 3.1.1 standardı](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html)
- [Eclipse Paho Go](https://github.com/eclipse-paho/paho.mqtt.golang)
- [Mosquitto yapılandırması](https://mosquitto.org/man/mosquitto-conf-5.html)

Topic adları, zaman bütçeleri ve cihaz davranışları bu projeye ait tasarım kararlarıdır.
