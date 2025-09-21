#include <Arduino.h>

#include <WiFi.h>
#include <HTTPClient.h>
#include <NetworkClientSecure.h>
#include <WiFiProvisioner.h>
#include <GxEPD2_7C.h>
#include <ArduinoJson.h>
#include <Preferences.h>
#include <x509_crt_bundle.h>
#include "esp_mac.h"

Preferences preferences;

const String DEVICE_NAME = "DEADBEEF";
const int DEVICE_WIDTH = 800;
const int DEVICE_HEIGHT = 480;
const int COLORS = 7;                              // Number of colors supported by the display
const String SERVER_URL = "https://10.0.0.4:8080"; // Replace with your server URL
const char *RESET_PASSWD = "password";             // Password to reset WiFi settings
const char *CONFIG_NAME = "PhotoFrame";
const char *TIME_SERVER = "10.0.0.1"; //NTP Server

// --- Sleep Parameters ---
#define uS_TO_S_FACTOR 1000000ULL // Conversion factor for micro seconds to seconds
#define TIME_TO_SLEEP 120         // Sleep time in seconds (adjust as needed)

// Device specific settings
const int TOUCH_PIN = 2; // Replace with the actual touch pin number
const int TOUCH_THRESHOLD = 40;
// Custom E-ink Adapter Board
const int SDI = 4;
const int CLK = 5;
const int CS = 6;
const int DC = 7;
const int RES = 15;
const int BUSY_PIN = 16;

const int SDO = 17; // NC

String bearer_token = "";

GxEPD2_7C<GxEPD2_730c_GDEY073D46, GxEPD2_730c_GDEY073D46::HEIGHT / 4> display(GxEPD2_730c_GDEY073D46(/*CS=5*/ CS, /*DC=*/DC, /*RST=*/RES, /*BUSY=*/BUSY_PIN)); // GDEY073D46 800x480 7-color, (N-FPC-001 2021.11.26)
SPIClass hspi(HSPI);

bool updateImage();
void drawFull(uint8_t *downloadedImages[]);
void setClock();
bool connectToWiFi();
String getMacAddress();
bool register_device();
bool download_and_display(JsonArray images);
void start_up();

// Forward declarations for JSON helper functions
bool parseJsonResponse(String response, JsonDocument &doc);

// HTTP helper functions
String httpsPOST(String url, String jsonPayload, String auth = "");
String httpsGET(String url, String auth = "");
uint8_t *downloadImage(String download_url);

void enableOutput()
{
  pinMode(CS, OUTPUT);
  pinMode(DC, OUTPUT);
  pinMode(RES, OUTPUT);
}

void setup()
{
  // put your setup code here, to run once:
  Serial.begin(115200);
  enableOutput();

  // Decide what to do based on wakeup cause
  esp_sleep_wakeup_cause_t wakeup_reason = esp_sleep_get_wakeup_cause();

  if (wakeup_reason == ESP_SLEEP_WAKEUP_TIMER)
  {
    Serial.println("Wakeup caused by timer");
    runRoutine();
  }
  else if (wakeup_reason == ESP_SLEEP_WAKEUP_TOUCHPAD)
  {
    Serial.println("Wakeup caused by touch");
    runTouchRoutine();
  }
  else
  {
    Serial.println("Power-on or reset");
    runRoutine();
  }
  goToSleep();
}

void loop()
{
}

void runRoutine()
{
  Serial.println("Running routine..");
  connect_wifi();
  getImage();
  Serial.println("\nRoutine complete.");
}

void runTouchRoutine()
{
  Serial.println("Touch detected! Running touch routine...");
  connect_wifi();
  updateImage();
  Serial.println("\nTouch routine complete.");
}

bool parseJsonResponse(String response, JsonDocument &doc)
{
  DeserializationError error = deserializeJson(doc, response);
  if (error)
  {
    Serial.printf("[JSON] Parse error: %s\n", error.c_str());
    return false;
  }
  return true;
}
void goToSleep()
{
  Serial.println("Configuring deep sleep...");

  // Configure wakeup sources
  esp_sleep_enable_timer_wakeup(TIME_TO_SLEEP * uS_TO_S_FACTOR);
  touchSleepWakeUpEnable(TOUCH_PIN, TOUCH_THRESHOLD); // NULL = no ISR, only for deep sleep wake
  esp_sleep_enable_touchpad_wakeup();

  Serial.println("Going to deep sleep now...");
  Serial.flush(); // Ensure all serial output is sent
  esp_deep_sleep_start();
}

void init_display()
{
  Serial.println("Init Display");
  hspi.begin(CLK, SDO, SDI, CS); // remap hspi for EPD (swap pins)
  display.epd2.selectSPI(hspi, SPISettings(4000000, MSBFIRST, SPI_MODE0));
  display.init(115200);
}

void connect_wifi()
{
  preferences.begin(CONFIG_NAME, true); // Read-only mode
  String savedSSID = preferences.getString("ssid", "");
  String savedPassword = preferences.getString("password", "");
  preferences.end();

  if (!savedSSID.isEmpty())
  {
    Serial.printf("Connecting to saved Wi-Fi: %s\n", savedSSID.c_str());
    if (savedPassword.isEmpty())
    {
      WiFi.begin(savedSSID.c_str());
    }
    else
    {
      WiFi.begin(savedSSID.c_str(), savedPassword.c_str());
    }

    unsigned long startTime = millis();
    while (WiFi.status() != WL_CONNECTED)
    {
      if (millis() - startTime > 10000)
      {
        Serial.println("Failed to connect to saved Wi-Fi.");
        break;
      }
      delay(500);
    }

    if (WiFi.status() == WL_CONNECTED)
    {
      Serial.printf("Successfully connected to %s\n", savedSSID.c_str());
      start_up();
      return;
    }
  }
  else
  {
    Serial.println("No saved Wi-Fi credentials found.");
  }

  WiFiProvisioner provisioner;

  provisioner.getConfig().AP_NAME = strdup((String("PhotoFrame-") + getMacAddressNoSep()).c_str());
  Serial.println("Provisioner AP Name: " + String(provisioner.getConfig().AP_NAME));
  provisioner.getConfig().HTML_TITLE = "PhotoFrame WiFi Provisioning";
  provisioner.getConfig().SHOW_INPUT_FIELD = true;
  provisioner.getConfig().INPUT_TEXT = "Reset Password";
  provisioner.getConfig().INPUT_LENGTH = 8;
  provisioner.getConfig().SHOW_RESET_FIELD = true;

  // Set the success callback
  provisioner
      .onInputCheck([](const char *input) -> bool
                    { return strcmp(input, RESET_PASSWD); })
      .onFactoryReset([]()
                      {
        preferences.begin(CONFIG_NAME, false);
        Serial.println("Factory reset triggered! Clearing preferences...");
        preferences.clear(); // Clear all stored credentials and API key
        preferences.end(); })
      .onSuccess([](const char *ssid, const char *password, const char *input)
                 {
        Serial.printf("Provisioning successful! SSID: %s\n", ssid);
        preferences.begin(CONFIG_NAME, false);
        // Store the credentials and API key in preferences
        preferences.putString("ssid", String(ssid));
        if (password) {
          preferences.putString("password", String(password));
        }
        preferences.end();
        Serial.println("Credentials saved.");
        start_up(); });

  Serial.println("Connecting to WiFi...");
  provisioner.startProvisioning();
}

void start_up()
{
  setClock();
  preferences.begin(CONFIG_NAME, true); // Read-only mode
  bearer_token = preferences.getString("bearer_token", "");
  preferences.end();

  if (bearer_token.isEmpty())
  {
    if (!register_device())
    {
      Serial.println("Device registration failed. Restarting...");
      delay(5000);
      ESP.restart();
    }
  }
  else
  {
    Serial.println("Bearer token OK");
  }
}

bool register_device()
{
  Serial.println("Registering device...");

  // Create JSON payload
  JsonDocument doc;
  doc["device_name"] = DEVICE_NAME;
  doc["device_id"] = getMacAddress();
  String jsonPayload;
  serializeJson(doc, jsonPayload);

  String response = httpsPOST(SERVER_URL + "/register", jsonPayload);
  if (response.isEmpty())
  {
    return false;
  }

  JsonDocument responseDoc;
  if (!parseJsonResponse(response, responseDoc))
  {
    Serial.println("Failed to parse JSON response:" + response);
    return false;
  }
  if (!responseDoc["success"])
  {
    Serial.println("[HTTPS] Registration failed:" + responseDoc["success"].as<String>());
    return false; // Registration failed
  }

  // Check if the registration was successful
  if (responseDoc["data"]["message"] != "Device registered")
  {
    Serial.println("[HTTPS] Registration failed:" + responseDoc["data"]["message"].as<String>());
    return false; // Registration failed
  }

  // Print the response
  Serial.println("[HTTPS] Registration successful");
  bearer_token = responseDoc["data"]["token"].as<String>();
  preferences.begin(CONFIG_NAME, false);
  preferences.putString("bearer_token", bearer_token);
  preferences.end();
  Serial.println("Bearer token saved: " + bearer_token);
  return true; // Registration successful
}

bool getImage()
{
  Serial.println("Getting image...");

  // Create JSON payload for image request

  JsonDocument doc;
  doc["action"] = "get_image";
  String jsonPayload;
  serializeJson(doc, jsonPayload);

  String response = httpsPOST(SERVER_URL + "/dev", jsonPayload, bearer_token);
  if (response.isEmpty())
  {
    return false;
  }

  JsonDocument responseDoc;
  if (!parseJsonResponse(response, responseDoc))
  {
    Serial.println("Failed to parse JSON response:" + response);
    return false;
  }

  if (!responseDoc["success"])
  {
    String error = responseDoc["error"].as<String>();
    Serial.println("[HTTPS] Update failed:" + error);
    return false; // Update failed
  }
  // Check if the update was successful
  if (responseDoc["data"]["message"] == "No image update needed")
  {
    Serial.println("[HTTPS] No image update needed");
    return true; // No update needed
  }
  // Print the response
  Serial.println("[HTTPS] Update successful");
  // Use the image data from the response
  JsonArray images = responseDoc["data"]["image"].as<JsonArray>();
  return download_and_display(images);
}

bool updateImage()
{
  Serial.println("Updating image...");

  // Create JSON payload for image update
  JsonDocument doc;
  doc["action"] = "update_image";
  String jsonPayload;
  serializeJson(doc, jsonPayload);

  String response = httpsPOST(SERVER_URL + "/dev", jsonPayload, bearer_token);
  if (response.isEmpty())
  {
  }

  JsonDocument responseDoc;
  if (!parseJsonResponse(response, responseDoc))
  {
    Serial.println("Failed to parse JSON response:" + response);
    return false;
  }

  if (!responseDoc["success"])
  {
    String error = responseDoc["error"].as<String>();
    Serial.println("[HTTPS] Update failed:" + error);
    return false; // Update failed
  }

  // Print the response
  Serial.println("[HTTPS] Update successful");
  // Use the image data from the response
  JsonArray images = responseDoc["data"]["image"].as<JsonArray>();
  return download_and_display(images);
}

bool download_and_display(JsonArray images)
{
  uint8_t *downloadedImages[COLORS] = {nullptr};
  int imageCount = 0;

  for (JsonVariant image : images)
  {
    if (imageCount >= COLORS)
      break;

    String download_url = image.as<String>();
    Serial.println("Downloading image " + String(imageCount + 1) + ": " + download_url);

    downloadedImages[imageCount] = downloadImage(download_url);
    if (downloadedImages[imageCount])
    {
      Serial.println("Image " + String(imageCount + 1) + " downloaded successfully");
      imageCount++;
    }
    else
    {
      Serial.println("Failed to download image " + String(imageCount + 1));
    }
  }

  // Print summary of downloaded images
  Serial.printf("Successfully downloaded %d images\n", imageCount);

  drawFull(downloadedImages);
  Serial.println("Display updated with new images");
  display.hibernate(); // Put the display to sleep to save power
  Serial.println("Display hibernated");

  // Free the downloaded image data when done using it
  for (int i = 0; i < imageCount; i++)
  {
    if (downloadedImages[i])
    {
      free(downloadedImages[i]);
      downloadedImages[i] = nullptr;
    }
  }

  return true; // Update successful
}

uint8_t *downloadImage(String download_url)
{
  Serial.println("Downloading image from: " + download_url);

  // Use the GET helper function to retrieve headers first to get content length
  NetworkClientSecure *client = new NetworkClientSecure;
  if (!client)
  {
    Serial.println("Failed to allocate NetworkClientSecure");
    return nullptr;
  }

  client->setCACertBundle(x509_crt_bundle, x509_crt_bundle_len);

  {
    HTTPClient https;
    if (https.begin(*client, SERVER_URL + "/" + download_url))
    {
      https.addHeader("Authorization", "Bearer " + bearer_token);

      int httpCode = https.GET();
      if (httpCode > 0 && httpCode == HTTP_CODE_OK)
      {
        int contentLength = https.getSize();
        Serial.printf("Content length: %d\n", contentLength);

        // Check if there's content to download
        if (contentLength > 0)
        {
          uint8_t *buffer = (uint8_t *)heap_caps_malloc(contentLength, MALLOC_CAP_SPIRAM);
          if (!buffer)
          {
            Serial.println("Failed to allocate PSRAM for image, falling back to regular memory");
            buffer = (uint8_t *)malloc(contentLength);
            if (!buffer)
            {
              Serial.println("Failed to allocate memory for image");
              https.end();
              delete client;
              return nullptr;
            }
          }
          else
          {
            Serial.println("Using PSRAM for image buffer");
          }

          WiFiClient *stream = https.getStreamPtr();
          size_t totalRead = 0;
          size_t bytesRead = 0;

          // Read all data
          while (https.connected() && (totalRead < contentLength))
          {
            if (stream->available())
            {
              bytesRead = stream->read(buffer + totalRead, contentLength - totalRead);
              totalRead += bytesRead;
              // Serial.printf("Downloaded: %d of %d bytes\n", totalRead, contentLength);
            }
            delay(1);
          }

          Serial.println("Download complete");
          https.end();
          delete client;
          return buffer;
        }
      }
      else
      {
        Serial.printf("[HTTPS] GET... failed, error: %s\n", https.errorToString(httpCode).c_str());
      }
      https.end();
    }
  }

  delete client;
  return nullptr;
}

// Helper function for making HTTPS POST requests
String httpsPOST(String url, String jsonPayload, String auth)
{
  NetworkClientSecure *client = new NetworkClientSecure;
  if (!client)
  {
    Serial.println("Failed to allocate NetworkClientSecure");
    return "";
  }

  client->setCACertBundle(x509_crt_bundle, x509_crt_bundle_len);

  {
    HTTPClient https;
    Serial.println("[HTTPS] begin...");
    if (https.begin(*client, url))
    {
      https.addHeader("Content-Type", "application/json");

      if (auth != "")
      {
        https.addHeader("Authorization", "Bearer " + auth);
      }

      Serial.println("[HTTPS] POST...");
      int httpCode = https.POST(jsonPayload);

      if (httpCode > 0)
      {
        Serial.printf("[HTTPS] POST... code: %d\n", httpCode);
        String payload = https.getString();
        https.end();
        delete client;
        return payload;
      }
      else
      {
        Serial.printf("[HTTPS] POST... failed, error: %s\n", https.errorToString(httpCode).c_str());
      }

      https.end();
    }
    else
    {
      Serial.println("[HTTPS] Unable to connect");
    }
  }

  delete client;
  return "";
}

// Helper function for making HTTPS GET requests
String httpsGET(String url, String auth)
{
  NetworkClientSecure *client = new NetworkClientSecure;
  if (!client)
  {
    Serial.println("Failed to allocate NetworkClientSecure");
    return "";
  }

  client->setCACertBundle(x509_crt_bundle, x509_crt_bundle_len);

  {
    HTTPClient https;
    Serial.println("[HTTPS] begin...");
    if (https.begin(*client, url))
    {

      if (auth != "")
      {
        https.addHeader("Authorization", "Bearer " + auth);
      }

      Serial.println("[HTTPS] GET...");
      int httpCode = https.GET();

      if (httpCode > 0)
      {
        Serial.printf("[HTTPS] GET... code: %d\n", httpCode);
        String payload = https.getString();
        https.end();
        delete client;
        return payload;
      }
      else
      {
        Serial.printf("[HTTPS] GET... failed, error: %s\n", https.errorToString(httpCode).c_str());
      }

      https.end();
    }
    else
    {
      Serial.println("[HTTPS] Unable to connect");
    }
  }

  delete client;
  return "";
}

void drawFull(uint8_t *downloadedImages[])
{
  init_display();
  display.firstPage();
  do
  {
    //clear the display
    display.fillScreen(GxEPD_WHITE);
    // Draw each color layer
    display.drawBitmap(0, 0, downloadedImages[0], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_BLACK);
    display.drawBitmap(0, 0, downloadedImages[1], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_WHITE);
    display.drawBitmap(0, 0, downloadedImages[2], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_BLUE);
    display.drawBitmap(0, 0, downloadedImages[3], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_GREEN);
    display.drawBitmap(0, 0, downloadedImages[4], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_RED);
    display.drawBitmap(0, 0, downloadedImages[5], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_YELLOW);
    display.drawBitmap(0, 0, downloadedImages[6], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_ORANGE);

  } while (display.nextPage()); // Repeat while there are pages left
  sleep(1);
  display.powerOff();
  Serial.println("Display update complete");
}

void setClock()
{
  configTime(0, 0, TIME_SERVER);

  Serial.print(F("Waiting for NTP time sync: "));
  time_t nowSecs = time(nullptr);
  while (nowSecs < 8 * 3600 * 2)
  {
    delay(500);
    Serial.print(F("."));
    yield();
    nowSecs = time(nullptr);
  }

  Serial.println();
  struct tm timeinfo;
  gmtime_r(&nowSecs, &timeinfo);
  Serial.print(F("Current time: "));
  Serial.print(asctime(&timeinfo));
}

// get MAC address
String getMacAddress()
{
  uint8_t mac[6];
  esp_efuse_mac_get_default(mac);
  String macStr = "";
  for (int i = 0; i < 6; i++)
  {
    char buf[3];
    sprintf(buf, "%02X", mac[i]);
    macStr += buf;
    if (i < 5)
    {
      macStr += ":";
    }
  }
  return macStr;
}
String getMacAddressNoSep()
{
  uint8_t mac[6];
  esp_efuse_mac_get_default(mac);
  String macStr = "";
  for (int i = 0; i < 6; i++)
  {
    char buf[3];
    sprintf(buf, "%02X", mac[i]);
    macStr += buf;
  }
  return macStr;
}