#include "rf_protocol.h"
#include <stdio.h>
#include <string.h>

namespace {
  constexpr uint8_t  MAGIC_0       = 0x53; // 'S'
  constexpr uint8_t  MAGIC_1       = 0x54; // 'T'
  constexpr uint16_t MOISTURE_NULL = 0xFFFF;

  void write_u16_le(uint8_t* p, uint16_t v) {
    p[0] = static_cast<uint8_t>(v & 0xFF);
    p[1] = static_cast<uint8_t>((v >> 8) & 0xFF);
  }

  void write_u32_le(uint8_t* p, uint32_t v) {
    for (int i = 0; i < 4; ++i) p[i] = static_cast<uint8_t>((v >> (8 * i)) & 0xFF);
  }

  void write_u64_le(uint8_t* p, uint64_t v) {
    for (int i = 0; i < 8; ++i) p[i] = static_cast<uint8_t>((v >> (8 * i)) & 0xFF);
  }

  uint16_t read_u16_le(const uint8_t* p) {
    return static_cast<uint16_t>(p[0]) | (static_cast<uint16_t>(p[1]) << 8);
  }

  uint32_t read_u32_le(const uint8_t* p) {
    uint32_t v = 0;
    for (int i = 0; i < 4; ++i) v |= (static_cast<uint32_t>(p[i]) << (8 * i));
    return v;
  }

  uint64_t read_u64_le(const uint8_t* p) {
    uint64_t v = 0;
    for (int i = 0; i < 8; ++i) v |= (static_cast<uint64_t>(p[i]) << (8 * i));
    return v;
  }

  uint32_t calc_crc32(const uint8_t* data, size_t len) {
    uint32_t crc = 0xFFFFFFFFu;
    for (size_t i = 0; i < len; ++i) {
      crc ^= data[i];
      for (int b = 0; b < 8; ++b) {
        crc = (crc & 1u) ? ((crc >> 1) ^ 0xEDB88320u) : (crc >> 1);
      }
    }
    return ~crc;
  }
}

bool rf_is_valid_direction(FrameType type, NodeId origin, NodeId destination) {
  switch (type) {
    case FrameType::TELEMETRY:
    case FrameType::HEARTBEAT:
    case FrameType::ACK:
      return (origin == NodeId::D1_SENSOR && destination == NodeId::D3_GATEWAY);
    case FrameType::START:
    case FrameType::PERMIT:
    case FrameType::STOP:
      return (origin == NodeId::D3_GATEWAY && destination == NodeId::D1_SENSOR);
    default:
      return false;
  }
}

bool rf_validate_frame(const RfFrame& f) {
  if (f.v != SafetyConfig::RF_FRAME_VERSION) return false;
  if (!rf_is_valid_direction(f.type, f.origin, f.destination)) return false;
  if (f.boot_id == 0) return false;

  // frame.schema.json: (hops=0,ttl=2) | (hops=1,ttl=1) | (hops=2,ttl=0)
  if (f.hops > 2 || f.ttl > 2 || (f.hops + f.ttl) != 2) return false;

  switch (f.type) {
    case FrameType::TELEMETRY:
      if (f.command_id != 0 || f.target_boot_id != 0) return false;
      if (f.adc_raw > 4095) return false;
      if (static_cast<uint8_t>(f.quality) > static_cast<uint8_t>(SensorQuality::SENSOR_FAULT)) return false;
      if (f.quality == SensorQuality::VALID) {
        if (!f.has_moisture_pct || f.moisture_pct_x100 > 10000 || f.calibration_revision < 1) return false;
      } else {
        if (f.has_moisture_pct) return false;
      }
      return true;

    case FrameType::HEARTBEAT:
      if (f.command_id != 0 || f.target_boot_id != 0) return false;
      if (f.arm_nonce == 0) return false;
      return true;

    case FrameType::START:
      if (f.command_id == 0 || f.run_id == 0 || f.target_boot_id == 0) return false;
      if (f.arm_nonce == 0) return false;
      if (f.duration_ms < 1 || f.duration_ms > SafetyConfig::PUMP_HARD_CAP_MS) return false;
      if (f.permit_budget_ms != SafetyConfig::PERMIT_BUDGET_MS) return false;
      return true;

    case FrameType::PERMIT:
      if (f.command_id == 0 || f.run_id == 0 || f.target_boot_id == 0) return false;
      if (f.arm_nonce == 0) return false;
      if (f.permit_budget_ms != SafetyConfig::PERMIT_BUDGET_MS) return false;
      return true;

    case FrameType::STOP:
      if (f.command_id == 0 || f.run_id == 0 || f.target_boot_id == 0) return false;
      return true;

    case FrameType::ACK:
      if (f.target_boot_id != 0 || f.command_id == 0 || f.run_id == 0) return false;
      if (static_cast<uint8_t>(f.ack_status) > static_cast<uint8_t>(AckStatus::FAULT)) return false;
      if (static_cast<uint8_t>(f.ack_reason) > static_cast<uint8_t>(ReasonCode::INVALID_FRAME)) return false;
      return true;

    default:
      return false;
  }
}

bool rf_encode(const RfFrame& f, uint8_t out_buf[SafetyConfig::WIRE_FRAME_BYTES]) {
  if (!out_buf || !rf_validate_frame(f)) return false;

  memset(out_buf, 0, SafetyConfig::WIRE_FRAME_BYTES);

  // Baslik [0..43]
  out_buf[0] = MAGIC_0;
  out_buf[1] = MAGIC_1;
  out_buf[2] = f.v;
  out_buf[3] = static_cast<uint8_t>(f.type);
  out_buf[4] = static_cast<uint8_t>(f.origin);
  out_buf[5] = static_cast<uint8_t>(f.destination);
  out_buf[6] = f.hops;
  out_buf[7] = f.ttl;
  write_u64_le(&out_buf[8],  f.boot_id);
  write_u32_le(&out_buf[16], f.seq);
  write_u64_le(&out_buf[20], f.command_id);
  write_u64_le(&out_buf[28], f.run_id);
  write_u64_le(&out_buf[36], f.target_boot_id);

  // Tipe ozel payload [44..75]
  switch (f.type) {
    case FrameType::TELEMETRY:
      write_u16_le(&out_buf[44], f.adc_raw);
      write_u16_le(&out_buf[46], f.has_moisture_pct ? f.moisture_pct_x100 : MOISTURE_NULL);
      out_buf[48] = static_cast<uint8_t>(f.quality);
      write_u16_le(&out_buf[49], f.calibration_revision);
      write_u32_le(&out_buf[51], f.sample_age_ms);
      write_u32_le(&out_buf[55], f.uptime_ms);
      break;

    case FrameType::HEARTBEAT:
      write_u64_le(&out_buf[44], f.arm_nonce);
      out_buf[52] = f.pump_output_on ? 1 : 0;
      out_buf[53] = f.fault_code;
      write_u16_le(&out_buf[54], f.calibration_revision);
      write_u32_le(&out_buf[56], f.uptime_ms);
      break;

    case FrameType::START:
      write_u64_le(&out_buf[44], f.arm_nonce);
      write_u32_le(&out_buf[52], f.duration_ms);
      write_u16_le(&out_buf[56], f.permit_budget_ms);
      break;

    case FrameType::PERMIT:
      write_u64_le(&out_buf[44], f.arm_nonce);
      write_u16_le(&out_buf[52], f.permit_budget_ms);
      break;

    case FrameType::STOP:
      // Payload bos
      break;

    case FrameType::ACK:
      out_buf[44] = static_cast<uint8_t>(f.ack_status);
      out_buf[45] = static_cast<uint8_t>(f.ack_reason);
      write_u16_le(&out_buf[46], f.elapsed_ms);
      write_u32_le(&out_buf[48], f.uptime_ms);
      break;

    default:
      return false;
  }

  // CRC-32 [76..79]
  write_u32_le(&out_buf[76], calc_crc32(out_buf, 76));
  return true;
}

bool rf_decode(const uint8_t* buf, size_t len, RfFrame& out) {
  if (!buf || len != SafetyConfig::WIRE_FRAME_BYTES) return false;
  if (buf[0] != MAGIC_0 || buf[1] != MAGIC_1) return false;
  if (read_u32_le(&buf[76]) != calc_crc32(buf, 76)) return false;

  out = RfFrame{};
  out.v              = buf[2];
  out.type           = static_cast<FrameType>(buf[3]);
  out.origin         = static_cast<NodeId>(buf[4]);
  out.destination    = static_cast<NodeId>(buf[5]);
  out.hops           = buf[6];
  out.ttl            = buf[7];
  out.boot_id        = read_u64_le(&buf[8]);
  out.seq            = read_u32_le(&buf[16]);
  out.command_id     = read_u64_le(&buf[20]);
  out.run_id         = read_u64_le(&buf[28]);
  out.target_boot_id = read_u64_le(&buf[36]);

  switch (out.type) {
    case FrameType::TELEMETRY: {
      out.adc_raw = read_u16_le(&buf[44]);
      uint16_t raw_pct = read_u16_le(&buf[46]);
      out.has_moisture_pct     = (raw_pct != MOISTURE_NULL);
      out.moisture_pct_x100    = out.has_moisture_pct ? raw_pct : 0;
      out.quality              = static_cast<SensorQuality>(buf[48]);
      out.calibration_revision = read_u16_le(&buf[49]);
      out.sample_age_ms        = read_u32_le(&buf[51]);
      out.uptime_ms            = read_u32_le(&buf[55]);
      break;
    }
    case FrameType::HEARTBEAT:
      out.arm_nonce            = read_u64_le(&buf[44]);
      out.pump_output_on       = (buf[52] != 0);
      out.fault_code           = buf[53];
      out.calibration_revision = read_u16_le(&buf[54]);
      out.uptime_ms            = read_u32_le(&buf[56]);
      break;

    case FrameType::START:
      out.arm_nonce        = read_u64_le(&buf[44]);
      out.duration_ms      = read_u32_le(&buf[52]);
      out.permit_budget_ms = read_u16_le(&buf[56]);
      break;

    case FrameType::PERMIT:
      out.arm_nonce        = read_u64_le(&buf[44]);
      out.permit_budget_ms = read_u16_le(&buf[52]);
      break;

    case FrameType::STOP:
      break;

    case FrameType::ACK:
      out.ack_status = static_cast<AckStatus>(buf[44]);
      out.ack_reason = static_cast<ReasonCode>(buf[45]);
      out.elapsed_ms = read_u16_le(&buf[46]);
      out.uptime_ms  = read_u32_le(&buf[48]);
      break;

    default:
      return false;
  }

  return rf_validate_frame(out);
}

bool rf_apply_hop(RfFrame& frame) {
  if (frame.ttl == 0 || frame.hops >= 2) return false;
  frame.hops += 1;
  frame.ttl -= 1;
  return true;
}

void rf_u64_to_hex16(uint64_t val, char out[17]) {
  snprintf(out, 17, "%016llx", static_cast<unsigned long long>(val));
}