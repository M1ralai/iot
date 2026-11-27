#pragma once
#include <stdint.h>

// shared/rules/safety-profile.json (go-mqtt-v1) sabitleri
namespace SafetyConfig {
  constexpr uint8_t  MQTT_SCHEMA_VERSION        = 1;
  constexpr uint8_t  RF_FRAME_VERSION           = 2;
  constexpr uint8_t  WIRE_FRAME_BYTES           = 80;

  constexpr uint32_t PUMP_HARD_CAP_MS           = 25000;
  constexpr uint16_t PERMIT_BUDGET_MS           = 3000;
  constexpr uint32_t NONCE_VALID_MS             = 3000;
  constexpr uint32_t HEARTBEAT_INTERVAL_MS      = 1000;
  constexpr uint32_t TELEMETRY_INTERVAL_MS      = 5000;
  constexpr uint32_t SAMPLE_MAX_AGE_MS          = 30000;
  constexpr uint32_t COOLDOWN_MS                = 1800000; // 30 dk
  constexpr uint32_t BOOT_LOCKOUT_MS            = 1800000; // 30 dk
  constexpr uint32_t COMMAND_DISPATCH_TTL_MS    = 5000;

  constexpr uint16_t MQTT_KEEPALIVE_SEC         = 15;
  constexpr uint32_t NODE_OFFLINE_AFTER_MS      = 5000;
  constexpr uint32_t GATEWAY_OFFLINE_AFTER_MS   = 5000;
  constexpr uint32_t RF_QUEUE_MAX_AGE_MS        = 1000;
  constexpr uint8_t  MAX_SEND_ATTEMPTS          = 3;
  constexpr uint32_t RETRY_BACKOFF_MS           = 100;
  constexpr uint16_t MAX_MQTT_PAYLOAD_BYTES     = 4096;

  constexpr uint16_t MAX_TERMINAL_RUNS_PER_BOOT = 128;
  constexpr uint8_t  MAX_PENDING_NONCES         = 4;
  constexpr uint8_t  LOW_MOISTURE_PCT           = 35;
  constexpr uint8_t  HIGH_MOISTURE_PCT          = 55;
}

enum class NodeId : uint8_t {
  UNKNOWN    = 0,
  D1_SENSOR  = 1,
  D2_RELAY   = 2,
  D3_GATEWAY = 3
};

// shared/rules/state-machine.json durumlari (D1)
enum class DeviceState : uint8_t {
  BOOT_LOCKOUT = 0,
  READY        = 1,
  RUNNING      = 2,
  COOLDOWN     = 3,
  FAULT        = 4
};

// shared/espnow/frame.schema.json -> telemetry.quality
enum class SensorQuality : uint8_t {
  VALID        = 0,
  UNCALIBRATED = 1,
  DISCONNECTED = 2,
  SATURATED    = 3,
  SENSOR_FAULT = 4
};

// shared/espnow/frame.schema.json -> ack.status
enum class AckStatus : uint8_t {
  ACCEPTED  = 0,
  APPLIED   = 1,
  STOPPED   = 2,
  COMPLETED = 3,
  REJECTED  = 4,
  FAULT     = 5
};

// shared/espnow/frame.schema.json -> ack.reason
enum class ReasonCode : uint8_t {
  OK            = 0,
  EXPIRED       = 1,
  BOOT_MISMATCH = 2,
  BUSY          = 3,
  MANUAL_STOP   = 4,
  LEASE_EXPIRED = 5,
  HARD_CAP      = 6,
  SENSOR_FAULT  = 7,
  COOLDOWN      = 8,
  INVALID_FRAME = 9
};