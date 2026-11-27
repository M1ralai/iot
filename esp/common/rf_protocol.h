#pragma once
#include <stddef.h>
#include <stdint.h>
#include "safety_config.h"

enum class FrameType : uint8_t {
  UNKNOWN   = 0,
  TELEMETRY = 1, // D1 -> D3
  HEARTBEAT = 2, // D1 -> D3
  START     = 3, // D3 -> D1
  PERMIT    = 4, // D3 -> D1
  STOP      = 5, // D3 -> D1
  ACK       = 6  // D1 -> D3
};

// shared/espnow/frame.schema.json (v2) ile birebir eslesen cerceve yapisi
struct RfFrame {
  // Ortak baslik (wire: 44 bayt)
  uint8_t   v              = SafetyConfig::RF_FRAME_VERSION;
  FrameType type           = FrameType::UNKNOWN;
  NodeId    origin         = NodeId::UNKNOWN;
  NodeId    destination    = NodeId::UNKNOWN;
  uint8_t   hops           = 0;
  uint8_t   ttl            = 2;
  uint64_t  boot_id        = 0;
  uint32_t  seq            = 0;
  uint64_t  command_id     = 0;
  uint64_t  run_id         = 0;
  uint64_t  target_boot_id = 0;

  // Payload alanlari (wire: 32 bayt, tipe gore serilestirilir)
  // Telemetry
  uint16_t      adc_raw              = 0;
  bool          has_moisture_pct     = false;
  uint16_t      moisture_pct_x100    = 0; // 0..10000 (ornek: 4255 = %42.55)
  SensorQuality quality              = SensorQuality::UNCALIBRATED;
  uint16_t      calibration_revision = 0;
  uint32_t      sample_age_ms        = 0;

  // Heartbeat / Start / Permit ortak
  uint64_t arm_nonce        = 0;
  bool     pump_output_on   = false;
  uint8_t  fault_code       = 0;
  uint32_t duration_ms      = 0;
  uint16_t permit_budget_ms = SafetyConfig::PERMIT_BUDGET_MS;

  // Ack / Ortak zaman
  AckStatus  ack_status  = AckStatus::ACCEPTED;
  ReasonCode ack_reason  = ReasonCode::OK;
  uint16_t   elapsed_ms  = 0;
  uint32_t   uptime_ms   = 0;
};

bool rf_is_valid_direction(FrameType type, NodeId origin, NodeId destination);
bool rf_validate_frame(const RfFrame& frame);
bool rf_encode(const RfFrame& frame, uint8_t out_buf[SafetyConfig::WIRE_FRAME_BYTES]);
bool rf_decode(const uint8_t* buf, size_t len, RfFrame& out_frame);
bool rf_apply_hop(RfFrame& frame);

// 64-bit kimlikleri 16 haneli kucuk harf hex metne cevirir (MQTT JSON icin)
void rf_u64_to_hex16(uint64_t val, char out[17]);