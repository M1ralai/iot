# Ortak iletişim sözleşmesi

Bu klasör, **Go backend ↔ MQTT broker ↔ D3 gateway** iletişimini ve **D3 ↔ D2 ↔ D1 ESP-NOW** kurallarını tanımlar. Backend ve firmware aynı sözleşmeye göre geliştirilecek. Tek gerçek sulama bölgesi `zone-a`, tek pompa düğümü `node-01` bulunur.

Kullanıcının güncel tercihi Go + MQTT’dir. Projedeki eski HTTP exchange / FastAPI tasarımından farklı olarak cihaz iletişiminde bu klasör esas alınır. Web paneli backend ile HTTP kullanabilir. Önceki HTML’deki serbest metin kimlik örnekleri yerine burada 64 bit kimliklerin 16 karakterli hex gösterimi sabitlenmiştir.

## Nereden başlanır?

| Dosya | İçerik |
|---|---|
| [mqtt/README.md](mqtt/README.md) | Topic’ler, mesaj yaşam döngüsü, bağlantı ve tekrar kuralları |
| [mqtt/topics.json](mqtt/topics.json) | Yayıncı, abone, QoS ve retained seçenekleri |
| [mqtt/message.schema.json](mqtt/message.schema.json) | MQTT mesajlarının JSON Schema tanımı |
| [mqtt/examples/](mqtt/examples/) | Telemetri, heartbeat, START, PERMIT, STOP, ACK ve durum örnekleri |
| [espnow/README.md](espnow/README.md) | ESP’ler arası paket biçimi ve yönlendirme |
| [espnow/frame.schema.json](espnow/frame.schema.json) | Çözülmüş RF paketinin JSON gösterimi |
| [espnow/frame-layout.json](espnow/frame-layout.json) | Sabit 80 baytlık paketin offset ve uzunlukları |
| [espnow/enums.json](espnow/enums.json) | Sayısal mesaj, hata ve sonuç kodları |
| [espnow/golden-vectors.json](espnow/golden-vectors.json) | Go/C++ codec’lerinin aynı baytları üretmesi için referanslar |
| [rules/device-rules.md](rules/device-rules.md) | D1, D2, D3 ve backend’in kabul/ret ve güvenlik kuralları |
| [rules/safety-profile.json](rules/safety-profile.json) | Süre, kapasite ve sulama eşikleri |
| [rules/state-machine.json](rules/state-machine.json) | D1 durum geçişleri |
| [tests/acceptance.md](tests/acceptance.md) | Firmware ve backend için davranış kabul senaryoları |
| [validate.py](validate.py) | Şema, fixture, topic ve byte referansı doğrulayıcısı |

## Sürüm ve birim kuralları

- MQTT zarfı `schema_version: 1`, topic kökü `irrigation/v1` olur.
- ESP-NOW uygulama paketi `v: 2` olur. Bu, ESP-NOW radyo protokolünün sürüm numarası değildir.
- Süreler `_ms` alanlarında milisaniye, UTC zamanlar RFC 3339 biçimindedir.
- Kimlikler 16 küçük hex karakterli string’dir. JavaScript’in sayı hassasiyetine güvenilmez. Sıfır değer yalnız açıkça izin verilen “yok” alanlarında kullanılır.
- JSON alan ve enum adları büyük/küçük harfe duyarlıdır. Bilinmeyen alan, tür veya sürüm reddedilir. Yinelenen JSON anahtarları da parser katmanında reddedilir.
- Nem, JSON’da en fazla iki ondalık basamaklı yüzde; RF’de yüzde × 100 tamsayısıdır. Geçersiz nem JSON’da `null`, RF’de `65535` olur.
- MQTT JSON’u RF’ye aynen gönderilmez. D3, sabit byte düzenine encode/decode yapar.

Alan eklemek de mevcut katı şemayla uyumsuzdur. Yayındaki v1 değiştirilmez; yeni topic/sürüm hazırlanır. RF byte düzeni değişirse RF sürümü de artırılır. Süre profilinin değişmesi, ilgili şema, doküman, örnek ve iki uygulamanın birlikte güncellenmesini gerektirir.

## Doğrulama

Proje kökünden:

```sh
.venv-docs/bin/python shared/validate.py
```

Başka ortamda Python 3 ve `jsonschema` 4.x ile `referencing` gerekir; proje kökündeki `requirements-docs.txt` bunların kurulumunu kapsar. Doğrulayıcı dosyaları kendi konumundan bulur, ağdan şema indirmez.

Bu paket sözleşme ve test verisidir; çalışan MQTT servisi, Go istemcisi veya ESP firmware’i değildir. Statik örneklerin her biri bağımsızdır; aynı kimlikleri paylaşan START/PERMIT/STOP dosyaları canlı olay sırası olarak yayımlanmamalıdır. `tests/acceptance.md` içindeki davranışlar henüz donanım üzerinde doğrulanmış sayılmaz.
