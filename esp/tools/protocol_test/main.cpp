#include <Arduino.h>
#include "rf_protocol.h"

static void run_tests() {
  Serial.println("\n=== KONTRAT UYUMLU ESP-NOW FRAME v2 TESTI ===");
  uint8_t wire[SafetyConfig::WIRE_FRAME_BYTES];
  RfFrame rx{};

  // Test 1: Telemetry (valid + %42.55)
  RfFrame tel{};
  tel.type = FrameType::TELEMETRY;
  tel.origin = NodeId::D1_SENSOR;
  tel.destination = NodeId::D3_GATEWAY;
  tel.hops = 0; tel.ttl = 2;
  tel.boot_id = 0x0102030405060708ULL;
  tel.seq = 12;
  tel.adc_raw = 2150;
  tel.has_moisture_pct = true;
  tel.moisture_pct_x100 = 4255;
  tel.quality = SensorQuality::VALID;
  tel.calibration_revision = 1;
  tel.sample_age_ms = 250;
  tel.uptime_ms = 9000;

  bool t1 = rf_encode(tel, wire) && rf_decode(wire, sizeof(wire), rx) &&
            rx.boot_id == tel.boot_id && rx.moisture_pct_x100 == 4255 &&
            rx.has_moisture_pct && rx.quality == SensorQuality::VALID;
  Serial.printf("Test 1 (Telemetry v2)     : %s\n", t1 ? "PASS" : "FAIL");

  // Test 2: Kalibresiz sensor icin moisture_pct doluysa reddedilmeli
  tel.quality = SensorQuality::UNCALIBRATED;
  bool t2 = !rf_encode(tel, wire);
  Serial.printf("Test 2 (Null Nem Kurali)  : %s\n", t2 ? "PASS" : "FAIL");

  // Test 3: START komutu (D3 -> D1)
  RfFrame st{};
  st.type = FrameType::START;
  st.origin = NodeId::D3_GATEWAY;
  st.destination = NodeId::D1_SENSOR;
  st.hops = 0; st.ttl = 2;
  st.boot_id = 0xAABBCCDDEEFF0011ULL;
  st.seq = 3;
  st.command_id = 0x1111222233334444ULL;
  st.run_id = 0x5555666677778888ULL;
  st.target_boot_id = 0x0102030405060708ULL;
  st.arm_nonce = 0x9999AAAABBBBCCCCULL;
  st.duration_ms = 10000;
  st.permit_budget_ms = 3000;

  bool t3 = rf_encode(st, wire) && rf_decode(wire, sizeof(wire), rx) &&
            rx.duration_ms == 10000 && rx.arm_nonce == st.arm_nonce;
  Serial.printf("Test 3 (START Komutu)     : %s\n", t3 ? "PASS" : "FAIL");

  // Test 4: CRC bozuk paket reddi
  wire[25] ^= 0xFF;
  bool t4 = !rf_decode(wire, sizeof(wire), rx);
  Serial.printf("Test 4 (CRC Reddi)        : %s\n", t4 ? "PASS" : "FAIL");

  // Test 5: 2-Hop / TTL kurali
  bool t5 = rf_apply_hop(st) && rf_apply_hop(st) && !rf_apply_hop(st) &&
            st.hops == 2 && st.ttl == 0;
  Serial.printf("Test 5 (2-Hop / TTL)      : %s\n", t5 ? "PASS" : "FAIL");
}

void setup() {
  Serial.begin(115200);
  delay(1500);
}

void loop() {
  run_tests();
  delay(5000);
}