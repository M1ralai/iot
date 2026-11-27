#pragma once
#include <stdint.h>
#include "d1_hardware.h"
#include "rf_protocol.h"

// Masa testi icin 1 (lockout/cooldown 5 sn). Teslim/saha icin 0 yap (30 dk).
#define D1_FAST_BENCH_TEST 1

struct AckEvent {
  bool       pending    = false;
  uint64_t   command_id = 0;
  uint64_t   run_id     = 0;
  AckStatus  status     = AckStatus::ACCEPTED;
  ReasonCode reason     = ReasonCode::OK;
  uint16_t   elapsed_ms = 0;
};

class D1Safety {
 public:
  bool begin(D1Hardware* hw);
  void tick(uint32_t now_ms);

  // Komut isleyiciler (ESP-NOW veya Seri Monitor testinden cagrilir)
  AckEvent handleStart(uint64_t command_id, uint64_t run_id, uint64_t target_boot_id,
                       uint64_t arm_nonce, uint32_t duration_ms, uint16_t permit_budget_ms,
                       uint32_t now_ms);
  AckEvent handlePermit(uint64_t command_id, uint64_t run_id, uint64_t target_boot_id,
                        uint64_t arm_nonce, uint16_t permit_budget_ms, uint32_t now_ms);
  AckEvent handleStop(uint64_t command_id, uint64_t run_id, uint64_t target_boot_id,
                      uint32_t now_ms);
  bool clearFault(uint32_t now_ms);

  // Durum ve telemetri okuyucular
  DeviceState   state() const { return state_; }
  const char*   stateName() const;
  uint64_t      latestNonce() const { return latest_nonce_; }
  SensorReading lastReading() const { return last_reading_; }
  uint8_t       faultCode() const { return fault_code_; }
  bool          popAutonomousAck(AckEvent& out);

 private:
  struct NonceSlot {
    uint64_t value      = 0;
    uint32_t created_ms = 0;
    bool     used       = true;
  };

  struct TerminalRun {
    uint64_t   run_id     = 0;
    AckStatus  status     = AckStatus::COMPLETED;
    ReasonCode reason     = ReasonCode::OK;
    uint16_t   elapsed_ms = 0;
  };

  uint32_t lockoutDurationMs() const {
#if D1_FAST_BENCH_TEST
    return 5000;
#else
    return SafetyConfig::BOOT_LOCKOUT_MS;
#endif
  }

  uint32_t cooldownDurationMs() const {
#if D1_FAST_BENCH_TEST
    return 5000;
#else
    return SafetyConfig::COOLDOWN_MS;
#endif
  }

  void generateNonce(uint32_t now_ms);
  bool consumeNonce(uint64_t nonce, uint32_t now_ms, uint32_t& out_created_ms);
  int  findTerminalRun(uint64_t run_id) const;
  bool recordTerminalRun(uint64_t run_id, AckStatus st, ReasonCode rsn, uint16_t elapsed);
  void stopPumpToCooldown(uint64_t cmd_id, AckStatus st, ReasonCode rsn, uint32_t now_ms, bool emit_ack);
  void enterFault(uint8_t code);

  D1Hardware*   hw_                 = nullptr;
  DeviceState   state_              = DeviceState::BOOT_LOCKOUT;
  uint32_t      state_entered_ms_   = 0;
  uint8_t       fault_code_         = 0;

  SensorReading last_reading_{};
  uint32_t      last_sample_ms_     = 0;

  NonceSlot     nonces_[SafetyConfig::MAX_PENDING_NONCES]{};
  uint8_t       nonce_head_         = 0;
  uint64_t      latest_nonce_       = 0;
  uint32_t      last_nonce_ms_      = 0;

  // Aktif sulama bilgileri
  uint64_t      active_cmd_id_      = 0;
  uint64_t      active_run_id_      = 0;
  uint32_t      run_start_ms_       = 0;
  uint32_t      requested_end_ms_   = 0;
  uint32_t      hard_cap_end_ms_    = 0;
  uint32_t      lease_end_ms_       = 0;

  // Boot basina en fazla 128 terminal run (sessiz silme yasak)
  TerminalRun   terminal_runs_[SafetyConfig::MAX_TERMINAL_RUNS_PER_BOOT]{};
  uint16_t      terminal_count_     = 0;

  AckEvent      auto_ack_{};
};