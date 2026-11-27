# Davranış kabul senaryoları

Bu liste uygulama geliştirildiğinde test edilecek davranışları tanımlar. `validate.py` bu testleri çalıştırmış gibi raporlamaz; yalnız şema, fixture ve referans baytlarını doğrular.

| Kimlik | Girdi / olay | Beklenen davranış |
|---|---|---|
| C01 | D1 boot | OFF, yeni boot_id, 30 dakika BOOT_LOCKOUT |
| C02 | Kaynak 0/2 paketi D2 ve D3’te alınır | D2 1/1, D3 2/0; aynı kaynak seq |
| C03 | D1 doğrudan D3’ten RF alır | Komşu MAC/yön kuralıyla ret |
| C04 | Aynı MQTT START iki kez gelir | Tek fiziksel başlangıç, aynı deadline, tekrar ACK |
| C05 | Aynı command_id ile farklı duration veya nonce | İçerik çakışması; ret |
| C06 | Süresi geçmiş veya kullanılmış nonce | START/PERMIT ret; deadline uzamaz |
| C07 | Nonce t=1000’de oluşur; PERMIT t=2500’de gelir | İzin sonu t=4000; t=5500 olmaz |
| C08 | Doğru run için STOP, sonra gecikmiş START | OFF, terminal run, START ret |
| C09 | Eski run STOP’u, başka run çalışıyor | Aktif run yanlışlıkla kapatılmaz |
| C10 | Broker/D2/Go durur, PERMIT kesilir | Son izin deadline’ında D1 yerel OFF; en geç hard cap |
| C11 | Sürekli yeni PERMIT gelir | İlk istenen süre ve 25 saniye sınırı değişmez |
| C12 | 25.001 ms istek veya force=true | Şema/alan denetiminde ret |
| C13 | Yanlış MQTT topic veya gateway session | START/PERMIT ret |
| C14 | Retained START gelir | D3 işlemez |
| C15 | Yeni boot sonrasında eski START | target_boot_id uyuşmaz; ret |
| C16 | Geçmiş telemetri tekrar bağlanınca akar | Kaydedilebilir; AUTO girdisi veya güncel veri olmaz |
| C17 | D1 ACK’i kaybolur | Backend pending/unknown; yeni run yok; heartbeat/STOP uzlaştırması |
| C18 | D1 fault veya NVS yazma hatası | OFF + FAULT; manuel START da ret |
| C19 | D1 COOLDOWN sırasında reset | Yeni boot kilidi; bekleme atlatılamaz |
| C20 | OFF ve RUNNING durumunda nem %45 | OFF başlamaz; RUNNING süre/izin uygunsa sürer |
| C21 | Nem %30, yağış %80 veya hava bilinmiyor | AUTO başlamaz; gerekçe görünür |
| C22 | Terminal run tablosu dolu | Yeni START ret; eski kayıtlar replay’e açılmaz |
| C23 | RF uzunluk/flags/padding/payload_len hatası | Decode aşamasında ret; çıkış değişmez |
| C24 | Go yeniden başlar | Eski queued START/PERMIT yeniden yayımlanmaz; OFF uzlaştırması gerekir |
| C25 | ACK/RF link teslimi var, su kabı boş | Çıkış gözlemi su akışı sonucu olarak raporlanmaz |
| C26 | uint32 uptime taşar | Monoton fark hesabı doğru; süre uzamaz |
| C27 | JSON yinelenen anahtar veya NaN | Parser seviyesinde ret |
| C28 | Yanlış/eskimiş boot veya oturum heartbeat’i | Güncel registry geri alınmaz; yeni START üretilmez |

Donanım testinde süreler logic analyzer, LED veya sürücü çıkışı kaydıyla ölçülür. İlk fiziksel deney kısa süreli ve gözetimli yapılır. Wi-Fi kanal, firmware sürümü, fixture kimliği ve gözlenen gecikme kanıt dosyasında bulunur.
