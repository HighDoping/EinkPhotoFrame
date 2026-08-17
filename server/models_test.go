package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeviceSettingAcceptsAdminJSON(t *testing.T) {
	var settings DeviceSetting
	err := json.Unmarshal([]byte(`{
		"img_update_interval": 600,
		"rotation": 90,
		"palette": "7Eink",
		"dither_algorithm": "Atkinson",
		"dither_strength": 0.8,
		"auto_brightness": false,
		"auto_contrast": true,
		"resize_method": "fill_white"
	}`), &settings)
	if err != nil {
		t.Fatal(err)
	}
	if settings.ImgUpdateInterval != 600 || settings.DitherStrength != 0.8 || settings.Palette != "7Eink" || settings.AutoBrightness || !settings.AutoContrast {
		t.Fatalf("settings were not decoded correctly: %+v", settings)
	}
}

func TestDeviceTelemetryUsesAdminJSONFields(t *testing.T) {
	lastSeen := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(DeviceTelemetry{DeviceID: "device-1", BatteryLevel: 87, LastSeen: lastSeen})
	if err != nil {
		t.Fatal(err)
	}

	var telemetry map[string]json.RawMessage
	if err := json.Unmarshal(payload, &telemetry); err != nil {
		t.Fatal(err)
	}
	if _, ok := telemetry["last_seen"]; !ok {
		t.Fatalf("admin response is missing last_seen: %s", payload)
	}
	if _, ok := telemetry["battery_level"]; !ok {
		t.Fatalf("admin response is missing battery_level: %s", payload)
	}
	if _, ok := telemetry["LastSeen"]; ok {
		t.Fatalf("admin response contains the legacy LastSeen field: %s", payload)
	}
}
