#pragma once
#include <stdint.h>
#include "safety_config.h"

// Gercek sensor bagli degilken 1, kapasitif sensor GPIO 34'e baglandiginda 0 yap
#define USE_SIMULATED_SENSOR 1

struct SensorReading {
  uint16_t      adc_raw              = 0;
  bool          has_moisture_pct     = false;
  uint16_t      moisture_pct_x100    = 0; // 0..10000 (ornek: 3333 = %33.33)
  SensorQuality quality              = SensorQuality::UNCALIBRATED;
  uint16_t      calibration_revision = 0;
  uint32_t      timestamp_ms         = 0;
};

class D1Hardware {
 public:
  bool begin();

  void setPump(bool on);
  bool isPumpOn() const { return pump_on_; }

  bool setCalibration(uint16_t dry_adc, uint16_t wet_adc);
  uint16_t calibrationRevision() const { return cal_rev_; }
  SensorReading readMoisture();
  void setSimulatedRawAdc(uint16_t raw) { sim_raw_adc_ = raw; }

  uint64_t bootId() const { return boot_id_; }
  bool isCooldownPending() const { return cooldown_pending_; }
  bool setCooldownPending(bool pending);

 private:
  static constexpr int PIN_PUMP = 26; // Breadboard LED (ileride role/MOSFET)
  static constexpr int PIN_SENSOR = 34; // ADC1 (Wi-Fi/ESP-NOW ile uyumlu)

  bool     pump_on_          = false;
  uint64_t boot_id_          = 0;
  bool     cooldown_pending_ = false;
  uint16_t dry_adc_          = 3000;
  uint16_t wet_adc_          = 1200;
  uint16_t cal_rev_          = 1;
  uint16_t sim_raw_adc_      = 2400; // ~%33.33 nem (kuru baslangic)
};