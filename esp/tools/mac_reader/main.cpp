#include <Arduino.h>
#include <WiFi.h>

void setup() {
  Serial.begin(115200);
  delay(1000);
  WiFi.mode(WIFI_STA);
}

void loop() {
  Serial.print("ESP32 STA MAC Adresi: ");
  Serial.println(WiFi.macAddress());
  delay(2000);
}