#include "d1_safety.h"
#include <Arduino.h>
#include <esp_system.h>

static bool timeReached(uint32_t now_ms, uint32_t deadline_ms) {
  return static_cast<int32_t>(now_ms - deadline_ms) >= 0;
}

bool D1Safety::begin(D1Hardware* hw) {
  hw_ = hw;
  hw_->setPump(false);
  uint32_t now = millis();
  state_ = DeviceState::BOOT_LOCKOUT;
  state_entered_ms_ = now;
  last_reading_ = hw_->readMoisture();
  last_sample_ms_ = now;
  generateNonce(now);
  return true;
}

const char* D1Safety::stateName() const {
  switch (state_) {
    case DeviceState::BOOT_LOCKOUT: return "BOOT_LOCKOUT";
    case DeviceState::READY:        return "READY";
    case DeviceState::RUNNING:      return "RUNNING";
    case DeviceState::COOLDOWN:     return "COOLDOWN";
    case DeviceState::FAULT:        return "FAULT";
    default:                        return "UNKNOWN";
  }
}

void D1Safety::generateNonce(uint32_t now_ms) {
  uint64_t n = (static_cast<uint64_t>(esp_random()) << 32) | static_cast<uint64_t>(esp_random());
  if (n == 0) n = 1;
  nonces_[nonce_head_] = NonceSlot{n, now_ms, false};
  nonce_head_ = static_cast<uint8_t>((nonce_head_ + 1) % SafetyConfig::MAX_PENDING_NONCES);
  latest_nonce_ = n;
  last_nonce_ms_ = now_ms;
}

bool D1Safety::consumeNonce(uint64_t nonce, uint32_t now_ms, uint32_t& out_created_ms) {
  if (nonce == 0) return false;
  for (uint8_t i = 0; i < SafetyConfig::MAX_PENDING_NONCES; ++i) {
    if (!nonces_[i].used && nonces_[i].value == nonce) {
      if ((now_ms - nonces_[i].created_ms) <= SafetyConfig::NONCE_VALID_MS) {
        nonces_[i].used = true;
        out_created_ms = nonces_[i].created_ms;
        return true;
      }
      nonces_[i].used = true; // suresi dolmus
      return false;
    }
  }
  return false;
}

int D1Safety::findTerminalRun(uint64_t run_id) const {
  for (uint16_t i = 0; i < terminal_count_; ++i) {
    if (terminal_runs_[i].run_id == run_id) return static_cast<int>(i);
  }
  return -1;
}

bool D1Safety::recordTerminalRun(uint64_t run_id, AckStatus st, ReasonCode rsn, uint16_t elapsed) {
  if (findTerminalRun(run_id) >= 0) return true;
  if (terminal_count_ >= SafetyConfig::MAX_TERMINAL_RUNS_PER_BOOT) return false;
  terminal_runs_[terminal_count_++] = TerminalRun{run_id, st, rsn, elapsed};
  return true;
}

void D1Safety::enterFault(uint8_t code) {
  hw_->setPump(false);
  state_ = DeviceState::FAULT;
  fault_code_ = code;
}

void D1Safety::stopPumpToCooldown(uint64_t cmd_id, AckStatus st, ReasonCode rsn,
                                  uint32_t now_ms, bool emit_ack) {
  hw_->setPump(false);
  uint32_t elapsed = now_ms - run_start_ms_;
  uint16_t elapsed_u16 = (elapsed > 65535u) ? 65535u : static_cast<uint16_t>(elapsed);

  recordTerminalRun(active_run_id_, st, rsn, elapsed_u16);

  if (emit_ack) {
    auto_ack_ = AckEvent{true, cmd_id, active_run_id_, st, rsn, elapsed_u16};
  }

  active_run_id_ = 0;
  active_cmd_id_ = 0;
  state_ = DeviceState::COOLDOWN;
  state_entered_ms_ = now_ms;
}

bool D1Safety::popAutonomousAck(AckEvent& out) {
  if (!auto_ack_.pending) return false;
  out = auto_ack_;
  auto_ack_.pending = false;
  return true;
}

void D1Safety::tick(uint32_t now_ms) {
  // 1) Her 1 sn'de yeni nonce uret
  if ((now_ms - last_nonce_ms_) >= SafetyConfig::HEARTBEAT_INTERVAL_MS) {
    generateNonce(now_ms);
  }

  // 2) Her 500 ms'de yerel sensoru ornekle (agdan bagimsiz)
  if ((now_ms - last_sample_ms_) >= 500) {
    last_reading_ = hw_->readMoisture();
    last_sample_ms_ = now_ms;
  }

  // 3) Durum makinesi gecisleri
  if (state_ == DeviceState::RUNNING) {
    if (last_reading_.quality != SensorQuality::VALID) {
      stopPumpToCooldown(active_cmd_id_, AckStatus::FAULT, ReasonCode::SENSOR_FAULT, now_ms, true);
      enterFault(1);
      return;
    }
    if (timeReached(now_ms, hard_cap_end_ms_)) {
      stopPumpToCooldown(active_cmd_id_, AckStatus::FAULT, ReasonCode::HARD_CAP, now_ms, true);
      return;
    }
    if (timeReached(now_ms, lease_end_ms_)) {
      stopPumpToCooldown(active_cmd_id_, AckStatus::FAULT, ReasonCode::LEASE_EXPIRED, now_ms, true);
      return;
    }
    if (timeReached(now_ms, requested_end_ms_)) {
      stopPumpToCooldown(active_cmd_id_, AckStatus::COMPLETED, ReasonCode::OK, now_ms, true);
      return;
    }
    // Ust nem esigi (%55.00 = 5500) -> otonom yerel durdurma
    if (last_reading_.has_moisture_pct &&
        last_reading_.moisture_pct_x100 >= (SafetyConfig::HIGH_MOISTURE_PCT * 100u)) {
      stopPumpToCooldown(active_cmd_id_, AckStatus::COMPLETED, ReasonCode::OK, now_ms, true);
      return;
    }
  } else if (state_ == DeviceState::BOOT_LOCKOUT) {
    if ((now_ms - state_entered_ms_) >= lockoutDurationMs() &&
        last_reading_.quality == SensorQuality::VALID) {
      if (hw_->isCooldownPending() && !hw_->setCooldownPending(false)) {
        enterFault(2);
        return;
      }
      state_ = DeviceState::READY;
      state_entered_ms_ = now_ms;
    }
  } else if (state_ == DeviceState::COOLDOWN) {
    if ((now_ms - state_entered_ms_) >= cooldownDurationMs() &&
        last_reading_.quality == SensorQuality::VALID) {
      if (!hw_->setCooldownPending(false)) {
        enterFault(2);
        return;
      }
      state_ = DeviceState::READY;
      state_entered_ms_ = now_ms;
    }
  }
}

AckEvent D1Safety::handleStart(uint64_t command_id, uint64_t run_id, uint64_t target_boot_id,
                               uint64_t arm_nonce, uint32_t duration_ms,
                               uint16_t permit_budget_ms, uint32_t now_ms) {
  if (target_boot_id != hw_->bootId()) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::BOOT_MISMATCH, 0};
  }
  if (command_id == 0 || run_id == 0 || duration_ms < 1 ||
      duration_ms > SafetyConfig::PUMP_HARD_CAP_MS ||
      permit_budget_ms != SafetyConfig::PERMIT_BUDGET_MS) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::INVALID_FRAME, 0};
  }

  // Tekrar gelen START: terminal run ise eski sonucu dondur
  int term_idx = findTerminalRun(run_id);
  if (term_idx >= 0) {
    const TerminalRun& tr = terminal_runs_[term_idx];
    return AckEvent{true, command_id, run_id, tr.status, tr.reason, tr.elapsed_ms};
  }

  // Aktif run tekrari: sureyi uzatmadan kabul don
  if (state_ == DeviceState::RUNNING && active_run_id_ == run_id) {
    uint32_t el = now_ms - run_start_ms_;
    return AckEvent{true, command_id, run_id, AckStatus::ACCEPTED, ReasonCode::OK,
                    static_cast<uint16_t>(el > 65535u ? 65535u : el)};
  }

  // 128 terminal run limiti doluysa yeni START reddedilir
  if (terminal_count_ >= SafetyConfig::MAX_TERMINAL_RUNS_PER_BOOT) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::BUSY, 0};
  }

  if (state_ == DeviceState::FAULT) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::SENSOR_FAULT, 0};
  }
  if (state_ == DeviceState::BOOT_LOCKOUT || state_ == DeviceState::COOLDOWN) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::COOLDOWN, 0};
  }
  if (state_ == DeviceState::RUNNING) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::BUSY, 0};
  }

  // Taze ve gecerli sensor kontrolu
  if (last_reading_.quality != SensorQuality::VALID ||
      (now_ms - last_reading_.timestamp_ms) > SafetyConfig::SAMPLE_MAX_AGE_MS) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::SENSOR_FAULT, 0};
  }

  // Tek kullanimlik nonce dogrulamasi
  uint32_t nonce_created_ms = 0;
  if (!consumeNonce(arm_nonce, now_ms, nonce_created_ms)) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::EXPIRED, 0};
  }

  // Pompa acilmadan ONCE NVS cooldown_pending yazilir
  if (!hw_->setCooldownPending(true)) {
    enterFault(3);
    return AckEvent{true, command_id, run_id, AckStatus::FAULT, ReasonCode::SENSOR_FAULT, 0};
  }

  active_cmd_id_    = command_id;
  active_run_id_    = run_id;
  run_start_ms_     = now_ms;
  requested_end_ms_ = now_ms + duration_ms;
  hard_cap_end_ms_  = now_ms + SafetyConfig::PUMP_HARD_CAP_MS;
  lease_end_ms_     = nonce_created_ms + SafetyConfig::PERMIT_BUDGET_MS;

  state_ = DeviceState::RUNNING;
  state_entered_ms_ = now_ms;
  hw_->setPump(true);

  return AckEvent{true, command_id, run_id, AckStatus::ACCEPTED, ReasonCode::OK, 0};
}

AckEvent D1Safety::handlePermit(uint64_t command_id, uint64_t run_id, uint64_t target_boot_id,
                                uint64_t arm_nonce, uint16_t permit_budget_ms, uint32_t now_ms) {
  if (target_boot_id != hw_->bootId()) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::BOOT_MISMATCH, 0};
  }
  if (permit_budget_ms != SafetyConfig::PERMIT_BUDGET_MS) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::INVALID_FRAME, 0};
  }
  // PERMIT durmus pompayi acamaz, yeni run baslatamaz
  if (state_ != DeviceState::RUNNING || active_run_id_ != run_id) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::EXPIRED, 0};
  }

  uint32_t nonce_created_ms = 0;
  if (!consumeNonce(arm_nonce, now_ms, nonce_created_ms)) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::EXPIRED, 0};
  }

  uint32_t new_lease = nonce_created_ms + SafetyConfig::PERMIT_BUDGET_MS;
  if (static_cast<int32_t>(new_lease - lease_end_ms_) > 0) {
    lease_end_ms_ = new_lease;
  }

  uint32_t el = now_ms - run_start_ms_;
  return AckEvent{true, command_id, run_id, AckStatus::APPLIED, ReasonCode::OK,
                  static_cast<uint16_t>(el > 65535u ? 65535u : el)};
}

AckEvent D1Safety::handleStop(uint64_t command_id, uint64_t run_id, uint64_t target_boot_id,
                              uint32_t now_ms) {
  // STOP icin nonce gerekmez
  if (target_boot_id != hw_->bootId()) {
    return AckEvent{true, command_id, run_id, AckStatus::REJECTED, ReasonCode::BOOT_MISMATCH, 0};
  }
  if (state_ == DeviceState::RUNNING && active_run_id_ == run_id) {
    uint32_t el = now_ms - run_start_ms_;
    uint16_t el_u16 = static_cast<uint16_t>(el > 65535u ? 65535u : el);
    stopPumpToCooldown(command_id, AckStatus::STOPPED, ReasonCode::MANUAL_STOP, now_ms, false);
    return AckEvent{true, command_id, run_id, AckStatus::STOPPED, ReasonCode::MANUAL_STOP, el_u16};
  }

  // START henuz gelmeden STOP geldiyse terminal run olarak isaretle
  recordTerminalRun(run_id, AckStatus::STOPPED, ReasonCode::MANUAL_STOP, 0);
  return AckEvent{true, command_id, run_id, AckStatus::STOPPED, ReasonCode::MANUAL_STOP, 0};
}

bool D1Safety::clearFault(uint32_t now_ms) {
  if (state_ != DeviceState::FAULT) return false;
  hw_->setPump(false);
  fault_code_ = 0;
  state_ = DeviceState::BOOT_LOCKOUT;
  state_entered_ms_ = now_ms;
  return true;
}