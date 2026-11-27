# ESP-NOW paket sözleşmesi

## Düğümler ve rota

| RF kimliği | Rol | İzinli komşu | İşlem |
|---|---|---|---|
| 1 / D1 | Sensör ve pompa | D2 | Telemetri/heartbeat/ACK üretir; START/PERMIT/STOP işler |
| 2 / D2 | Tekrarlayıcı | D1 ve D3 | Sabit yolda iki yönde aktarır; pompa kararı vermez |
| 3 / D3 | Gateway | D2 | RF ile MQTT arasında dönüşüm yapar |

D1-D3 doğrudan peer kaydı yoktur. Komşu MAC allowlist’i ve encrypted unicast peer anahtarları kurulumda sağlanır. Gerçek anahtar ve MAC adresleri bu klasöre konmaz. Her hop gerçek gönderen MAC’iyle doğrulanır; header’daki `origin` tek başına kimlik kanıtı değildir. D2 güvenilir bileşendir; bu sürüm kötü niyetli D2’ye karşı uçtan uca imza sağlamaz.

D3’ün erişim noktasındaki 2,4 GHz Wi-Fi kanalı D1 ve D2 ile aynı olmalıdır. Kanal değişirse saha tekrar yapılandırılır. [Espressif ESP-NOW açıklaması](https://github.com/espressif/esp-faq/blob/master/docs/en/application-solution/esp-now.rst) birlikte kullanım koşulunu belirtir.

## Byte düzeni

[frame-layout.json](frame-layout.json) kaynak tanımdır. Toplam **80 bayt**: 48 bayt header ve 32 bayt payload alanı. Çok baytlı tamsayılar little-endian yazılır. Ham C++ `struct` belleği gönderilmez. Go ve C++ uygulamaları aynı offset’lere açık encode/decode yapar.

| Alan | Offset | Bayt |
|---|---:|---:|
| magic = ASCII `IR` | 0 | 2 |
| version = 2, type | 2, 3 | birer 1 |
| origin, destination, hops, ttl | 4, 5, 6, 7 | birer 1 |
| boot_id | 8 | 8 |
| seq | 16 | 4 |
| command_id, run_id, target_boot_id | 20, 28, 36 | sekizer 8 |
| payload_len, flags=0, reserved=0 | 44, 45, 46 | 1, 1, 2 |
| payload, artanı sıfır | 48 | 32 |

| Tür | Kod | Payload alan sırası | Kullanılan bayt |
|---|---:|---|---:|
| telemetry | 1 | adc u16, nem×100 u16, quality u8, calibration u16, sample_age u32, uptime u32 | 15 |
| heartbeat | 2 | arm_nonce u64, pump_output u8, fault u8, calibration u16, uptime u32 | 16 |
| start | 3 | arm_nonce u64, duration_ms u16, permit_budget_ms u16 | 12 |
| permit | 4 | arm_nonce u64, permit_budget_ms u16 | 10 |
| stop | 5 | boş | 0 |
| ack | 6 | status u8, reason u8, elapsed_ms u16, uptime u32 | 8 |

Tam 80 bayt olmayan, magic/sürüm/type uyuşmayan, payload_len yanlış veya ayrılmış baytları sıfır olmayan paket işlenmez. Nem, quality ve enum kontrolü decode sırasında yapılır. Bilinmeyen fault kodu güvenli tarafta değerlendirilir ve START engellenir. Kaynak JSON şeması bu kontrollerin yerini almaz.

## Hop, TTL ve tekrarlar

Kaynakta `hops=0, ttl=2`; D2 alımında `1/1`; hedef alımında `2/0` olur. Alıcı önce gelen konumu doğrular: D2 yalnız kaynak `0/2`, D1/D3 yalnız D2’den `1/1` kabul eder. Kabul edilen alımda hop artırılır, TTL azaltılır. Hedefte sıfıra inen TTL teslimi engellemez; hedef yeniden forward etmez.

Transport tekrar anahtarı `(origin, boot_id, seq)` olur. Aynı kimlikle farklı byte içeriği reddedilir. Sıra numarası tek boot boyunca artar; eski/out-of-order seq yeni komut sayılmaz. Seq taşmadan yeni boot epoch ile güvenli bağlantı kurulmalıdır. Süre karşılaştırmaları wrap güvenli monoton zamanla yapılır; 32 bit uptime normal zaman sırası diye karşılaştırılmaz.

Link tekrarı aynı transport seq ile yapılabilir; D2 önceden teslim edemediği pending paketin kendi sınırlı tekrarını sürdürebilir. Kaybolan uygulama ACK’ini almak için uçtan uca yeniden denemede yeni RF seq ve aynı command/run kimliği kullanılır. D1’in anlamsal tekrar kontrolü ikinci kez çalıştırmayı engeller.

Her gönderimde en fazla 3 deneme, başlangıç için 100 ms backoff ve kontrol kuyruğunda en fazla 1.000 ms bekleme vardır. Bu süreler hedef ölçümlerdir. Kuyruk dolarsa STOP öncelikli işlenir, START/PERMIT süresi uzatılmaz. Callback yalnız doğrulama ve kuyruğa alma yapar; ağ beklerken yerel pompa zamanlayıcısı çalışmaya devam eder.

## JSON ve RF dönüşümü

`frame.schema.json`, çözülmüş RF paketini tanımlar; radyo üzerinde JSON taşındığı anlamına gelmez. D3 MQTT komutunu RF’ye çevirirken `origin=3`, `destination=1`, `boot_id=D3 boot`, yeni seq, `hops=0`, `ttl=2` yazar; komut kimliği, run, hedef D1 boot ve payload korunur.

Telemetry/heartbeat header’ında `command_id=0` olur. `run_id`, aktif çalışmayı veya cooldown sırasında son çalışmayı gösterebilir; ilk boot’ta sıfırdır. ACK ilgili run ve command kimliklerini taşır. Tüm yukarı yönlü mesajlarda `target_boot_id=0` olur.

`golden-vectors.json` altı bağımsız örneğin beklenen byte dizisini içerir. Gateway MQTT örneklerinde aynı paketler iki alım tamamlandığı için `hops=2, ttl=0` biçimindedir. Kaynak RF örnekleri `0/2` kalır.
