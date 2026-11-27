# Cihazların ve backend’in uygulayacağı kurallar

“Zorunlu” ifadeler firmware/backend kabul ölçütüdür. Sayısal kaynak [safety-profile.json](safety-profile.json), D1 geçiş kaynağı [state-machine.json](state-machine.json) dosyasıdır.

## D1: sensör ve pompa

1. Reset ve boot sırasında sürücü çıkışı OFF olur. Her yeniden açılıştan sonra 30 dakika BOOT_LOCKOUT uygulanır; yeniden başlatma cooldown’u atlatamaz. İlk devreye alma da buna dahildir. Bu tercih önceki taslaktaki koşullu boot kilidinden daha basit ve daha sıkıdır.
2. Geçerli kalibrasyon, taze yerel sensör ölçümü ve hatasız durum olmadan START kabul edilmez. %0 nem tek başına kopuk sensör kanıtı değildir. ADC sınırı, bağlantı ve kalibrasyon ayrı değerlendirilir.
3. START yalnız READY durumunda, doğru hedef boot, yeni run, yeni command ve D1’in henüz süresi dolmamış nonce’u ile kabul edilir. D1 cooldown, süre ve nonce denetimini Go’dan bağımsız yapar.
4. D1 saniyede bir yeni, sıfır olmayan nonce üretir; üretim anını monoton saatle saklar. En çok dört güncel nonce tutulur. Geçerlilik nonce’un oluşmasından itibaren 3.000 ms’dir; eski nonce silinir. Nonce gizli anahtar değildir, tazelik kanıtıdır.
5. Bir nonce yalnız bir START veya PERMIT için kullanılabilir. Tekrar komut, önceki sonucu döndürür; yeni süre veya yeni çıkış değişimi üretmez. Yeni command_id ile aynı nonce’u kullanmak da süreyi uzatamaz.
6. Çıkış açılmadan NVS’ye `cooldown_pending` yazılır. Yazma hatasında OFF + FAULT. Durma sonrasında cooldown tamamlanmadan bu kayıt temizlenmez. Her boot kilidi uygulandığı için enerji kesmek de sınırı aşamaz.
7. İstenen süre 1..25.000 ms aralığındadır. Son durma zamanı, ilk START + istenen süredir. 25.000 ms mutlak sınır, ilk START anından hesaplanır. START tekrarı veya PERMIT bunları değiştiremez.
8. Çalışma izninin sonu `nonce_created_at + 3.000 ms` olur. Yeni izin yalnız aktif aynı run için alınabilir. PERMIT kapalı pompayı açmaz, yeni run oluşturmaz ve önceki izinden daha erken bir deadline’a gerilemez.
9. STOP, nonce gerektirmez. Doğru boot/run için OFF uygulanır; START henüz gelmediyse run için terminal kayıt tutulur. Daha sonra o run’ın START’ı gelirse reddedilir. Eski run’ın STOP’u başka aktif run’ı etkilemez.
10. Süre, izin, yerel sensör hatası veya üst nem eşiği %55 durmayı gerektirirse ağdan komut beklenmez. Ağ kütüphanesindeki bloklama, yerel ölçüm ve kapanmayı engellememelidir. Yazılım zamanlayıcısı röle yapışmasına karşı fiziksel koruma değildir.
11. Normal durma sonrası 30 dakika COOLDOWN; sensör/depolama/yapılandırma hatasında FAULT uygulanır. Hata giderilince operatör doğrulamasıyla BOOT_LOCKOUT’a dönülür. Otomatik sınırsız hata temizleme yapılmaz.
12. Bir boot boyunca en fazla 128 terminal run kimliği saklanır; dolunca yeni START reddedilir. Eski run kimlikleri sessizce silinip replay kapısı açılmaz. Bakım/reset sonrası boot değişir ve boot kilidi devreye girer. Aktif run ve son uygulanan komut sonuçları ayrıca tutulur.

START için `rejected` gerekçeleri `expired`, `boot_mismatch`, `busy`, `sensor_fault`, `cooldown` veya `invalid_frame` olabilir. Yerel izin/hard cap duruşu `fault` sonucu ve `lease_expired` / `hard_cap` gerekçesiyle bildirilebilir. İstenen süre veya hedef nem ile normal bitiş `completed/ok`, STOP `stopped/manual_stop` olur. Ret paketleri yalnız doğrulanmış komut gönderenine döner; bilinmeyen MAC’e yanıt verilmez.

## D2: tekrarlayıcı

D2 sulama kararı vermez, nonce üretmez, komut süresini değiştirmez. Telemetri/heartbeat/ACK’i yalnız D1’den D3’e; START/PERMIT/STOP’u yalnız D3’ten D1’e taşır. Bilinmeyen peer, yanlış yön, bozuk paket, yanlış hop/TTL ve tekrarlar elenir. Forward sırasında yalnız hop ve TTL değişir.

Bir next-hop link teslimi, D1 uygulama ACK’i yerine kullanılamaz. D2 kapanınca alternatif rota yoktur. Geri gelince eski kontrol kuyruğu oynatılmaz; taze trafik beklenir.

## D3: ağ geçidi

D3 MQTT ve RF sözleşmesini birbirine dönüştürür. Kimlik, topic, şema, payload boyutu, süre ve yön doğrulamasını yapar. Telemetriye ölçmediği RSSI veya UTC sensör zamanı eklemez. Alınamayan RSSI `null` olur.

MQTT gövdesi en fazla 4.096 UTF-8 bayt olabilir. Kontrol JSON’unda yinelenen anahtar, NaN/Infinity ve şemada olmayan alan reddedilir. Mesajın geçerli olması çalıştırılacağı anlamına gelmez; D1 son kararı verir.

Bağlantı kesilince START/PERMIT kuyruğu atılır. Gateway tek başına yeni izin veremez. Yeniden bağlanınca yeni gateway_session_id kullanır. Yeni oturuma ait taze D1 heartbeat’i Go’ya ulaşmadan START akışı açılmaz. D1 eski boot kimliğiyle gelen komutu ayrıca reddeder.

## Go backend

- Telemetri tekrarı `(node_id, frame.boot_id, frame.seq)` üzerinden ayıklanır. Aynı anahtarla farklı veri hata olarak kaydedilir.
- `message_id` yalnız MQTT teslim tekrarını ayırt eder; run idempotency ve RF seq kontrolünün yerini almaz.
- Tek fiziksel bölge ve tek aktif/belirsiz run kabul edilir. `zone-b` simülasyonu donanım komutu üretemez.
- Komut önce kalıcı kayda yazılır, sonra yayımlanır. MQTT PUBACK gelmesi cihaz durumunu `applied` yapmaz. D1 ACK’i veya aynı boot/run içeren taze heartbeat gerekir.
- Sonuç bilinmiyorsa `unknown` gösterilir, yeni START kilitlenir. ACK kaybında run terminal ilan edilmez; STOP/heartbeat ile uzlaştırılır.
- Manuel STOP, backend’de kalıcı kontrol kilididir. Kullanıcı açıkça kaldırmadan AUTO/manuel START çalışmaz. Manuel START tek seferliktir; yerel güvenliği ve cooldown’u atlamaz.
- AUTO: OFF durumunda nem <%35 ve yağmur göstergesi <%60 ise güvenlik kontrollerinden sonra START. RUNNING durumunda %35-%55 bandı mevcut çalışmayı korur; ≥%55 için STOP. Yerel süre ve izin sınırları her zaman önceliklidir.
- Önümüzdeki 6 saatin saatlik yağış olasılıklarının en büyüğü kullanılır. Bu, birleşik altı saatlik yağış olasılığı değildir. Hava saatte bir yenilenir; iki saati aşan veya eksik veri otomatik başlatmayı engeller. Manuel istek yağmur kuralını aşabilir, cihaz korumalarını aşamaz.
- D1, D3 veya backend yeniden başladığında eski START otomatik tekrar yayımlanmaz. Eski boot/oturumlar tek gecikmiş mesajla yeniden etkinleştirilmez.

Backend durumu `pending/running/stopping/completed/rejected/unknown` gibi uygulama durumları içerebilir; bunlar doğrudan RF ACK enum’u değildir. Yerel D1 durumu, MQTT bağlantısı, broker teslimi ve fiziksel su akışı ayrı kavramlardır.

## Uygulamaya aktarım

Go ve C++ tarafı aynı JSON sabitlerini elle kopyalayacaksa sürüm incelemesinde karşılaştırılır; ileride bu dosyalardan tip/sabit üretilebilir. Bu teslimde kod üretici veya firmware içine JSON Schema yorumlayıcısı şart koşulmaz. Firmware sabit alan ve sınır kontrollerini uygular; backend katı JSON çözümleme yapar.
