#include <Arduino.h>
#include "d1_hardware.h"
#include "d1_safety.h"

D1Hardware hw;
D1Safety   safety;
uint64_t   test_run_seq = 100;
uint64_t   test_cmd_seq = 1;
uint32_t   last_print_ms = 0;

static void printAck(const char* tag, const AckEvent& a) {
  Serial.printf("[%s] run=%llu status=%u reason=%u elapsed=%ums\n",
                tag, static_cast<unsigned long long>(a.run_id),
                static_cast<unsigned>(a.status), static_cast<unsigned>(a.reason),
                a.elapsed_ms);
}

void setup() {
  Serial.begin(115200);
  delay(1000);
  if (!hw.begin() || !safety.begin(&hw)) {
    Serial.println("HATA: D1 baslatilamadi!");
    while (true) delay(1000);
  }
  Serial.println("\n=== D1 GUVENLIK DURUM MAKINESI (TEST MODU: 5 sn) ===");
  Serial.println("Komutlar: start | permit | stop | wet | dry | status");
}

void loop() {
  uint32_t now = millis();
  safety.tick(now);

  // Otonom durdurma olayi (lease_expired, hard_cap, %55 nem vb.)
  AckEvent auto_ev;
  if (safety.popAutonomousAck(auto_ev)) {
    printAck("OTONOM STOP", auto_ev);
  }

  // Seri Monitor'den komut oku
  if (Serial.available()) {
    String cmd = Serial.readStringUntil('\n');
    cmd.trim();
    if (cmd == "start") {
      test_run_seq++;
      AckEvent a = safety.handleStart(test_cmd_seq++, test_run_seq, hw.bootId(),
                                      safety.latestNonce(), 10000, 3000, now);
      printAck("START ACK", a);
    } else if (cmd == "permit") {
      AckEvent a = safety.handlePermit(test_cmd_seq++, test_run_seq, hw.bootId(),
                                       safety.latestNonce(), 3000, now);
      printAck("PERMIT ACK", a);
    } else if (cmd == "stop") {
      AckEvent a = safety.handleStop(test_cmd_seq++, test_run_seq, hw.bootId(), now);
      printAck("STOP ACK", a);
    } else if (cmd == "wet") {
      hw.setSimulatedRawAdc(1800); // ~%66.67 nem (> %55)
      Serial.println("Simule nem %66.67 yapildi (otonom durdurma tetiklenmeli)");
    } else if (cmd == "dry") {
      hw.setSimulatedRawAdc(2400); // ~%33.33 nem
      Serial.println("Simule nem %33.33 yapildi");
    } else if (cmd == "status") {
      SensorReading r = safety.lastReading();
      Serial.printf("Durum=%s | Pompa=%s | Nem=%%%u.%02u | Nonce=%016llx\n",
                    safety.stateName(), hw.isPumpOn() ? "ON" : "OFF",
                    r.moisture_pct_x100 / 100, r.moisture_pct_x100 % 100,
                    static_cast<unsigned long long>(safety.latestNonce()));
    }
  }

  if (now - last_print_ms >= 2000) {
    last_print_ms = now;
    SensorReading r = safety.lastReading();
    Serial.printf("Durum=%-12s | Pompa=%-3s | Nem=%%%u.%02u\n",
                  safety.stateName(), hw.isPumpOn() ? "ON" : "OFF",
                  r.moisture_pct_x100 / 100, r.moisture_pct_x100 % 100);
  }
}