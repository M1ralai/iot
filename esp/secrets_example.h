#pragma once
#include <stdint.h>

// Wi-Fi ve Mosquitto bilgileri (sadece D3 Gateway kullanacak)
static const char* WIFI_SSID = "WIFI_SSID";
static const char* WIFI_PASSWORD = "WIFI_PASSWORD";
static const uint8_t WIFI_CHANNEL = 6; // D1, D2 ve D3 ayni kanali kullanacak

static const char* MQTT_HOST = "192.168.1.100";
static const uint16_t MQTT_PORT = 1883;
static const char* MQTT_USER = "gw-01";
static const char* MQTT_PASSWORD = "MQTT_SIFRENIZ";

// 3 ESP32 kartinin donanim MAC adresleri
static const uint8_t MAC_D1_SENSOR[6] = {0x00, 0x00, 0x00, 0x00, 0x00, 0x00};
static const uint8_t MAC_D2_RELAY[6]   = {0x00, 0x00, 0x00, 0x00, 0x00, 0x00};
static const uint8_t MAC_D3_GATEWAY[6] = {0x00, 0x00, 0x00, 0x00, 0x00, 0x00};