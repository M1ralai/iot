#include "d1_hardware.h"
#include <Arduino.h>
#include <Preferences.h>
#include <esp_system.h>

static Preferences nvs;

bool D1Hardware::begin() {
  // Kural: Reset/boot aninda pompa cikisi derhal OFF zorlanir
  pinMode(PIN_PUMP, OUTPUT);
  digitalWrite(PIN_PUMP, LOW);
  pump_on_ = false;

  analogReadResolution(12); // 0..4095

  if (!nvs.begin("d1_safety", false)) {
    return false;
  }

  // Her yeniden baslatmada benzersiz, sifir olmayan 64-bit boot_id uret
  uint32_t boot_cnt = nvs.getUInt("boot_cnt", 0) + 1;
  if (nvs.putUInt("boot_cnt", boot_cnt) == 0) {
    return false;
  }
  uint32_t rnd = esp_random();
  boot_id_ = (static_cast<uint64_t>(boot_cnt) << 32) | static_cast<uint64_t>(rnd);
  if (boot_id_ == 0) boot_id_ = 1;

  cooldown_pending_ = nvs.getBool("cd_pend", false);
  dry_adc_          = nvs.getUShort("dry_adc", 3000);
  wet_adc_          = nvs.getUShort("wet_adc", 1200);
  cal_rev_          = nvs.getUShort("cal_rev", 1);
  return true;
}

void D1Hardware::setPump(bool on) {
  pump_on_ = on;
  digitalWrite(PIN_PUMP, on ? HIGH : LOW);
}

bool D1Hardware::setCalibration(uint16_t dry_adc, uint16_t wet_adc) {
  if (wet_adc == dry_adc) return false;
  uint16_t next_rev = (cal_rev_ == 65535) ? 1 : static_cast<uint16_t>(cal_rev_ + 1);
  if (nvs.putUShort("dry_adc", dry_adc) == 0) return false;
  if (nvs.putUShort("wet_adc", wet_adc) == 0) return false;
  if (nvs.putUShort("cal_rev", next_rev) == 0) return false;
  dry_adc_ = dry_adc;
  wet_adc_ = wet_adc;
  cal_rev_ = next_rev;
  return true;
}

bool D1Hardware::setCooldownPending(bool pending) {
  if (nvs.putBool("cd_pend", pending) == 0) {
    return false; // NVS yazma hatasi -> cagiran FAULT + OFF yapmali
  }
  cooldown_pending_ = pending;
  return true;
}

SensorReading D1Hardware::readMoisture() {
  SensorReading r{};
  r.timestamp_ms = millis();
  r.calibration_revision = cal_rev_;

  if (cal_rev_ < 1 || wet_adc_ == dry_adc_) {
    r.quality = SensorQuality::UNCALIBRATED;
    r.has_moisture_pct = false;
    return r;
  }

#if USE_SIMULATED_SENSOR
  r.adc_raw = sim_raw_adc_;
#else
  uint32_t sum = 0;
  for (int i = 0; i < 8; ++i) {
    sum += analogRead(PIN_SENSOR);
  }
  r.adc_raw = static_cast<uint16_t>(sum / 8);
#endif

  // Formul: clamp(100 * (raw - dry) / (wet - dry), 0, 100), 0.01 hassasiyet
  float pct = 100.0f * (static_cast<float>(r.adc_raw) - static_cast<float>(dry_adc_)) /
              (static_cast<float>(wet_adc_) - static_cast<float>(dry_adc_));
  if (pct < 0.0f) pct = 0.0f;
  if (pct > 100.0f) pct = 100.0f;

  r.moisture_pct_x100 = static_cast<uint16_t>(pct * 100.0f + 0.5f);
  r.has_moisture_pct = true;
  r.quality = SensorQuality::VALID;
  return r;
}