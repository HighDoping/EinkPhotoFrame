#include <Arduino.h>

#include <WiFi.h>
#include <HTTPClient.h>
#include <NetworkClientSecure.h>
#include <WiFiProvisioner.h>

#include <GxEPD2_7C.h>
GxEPD2_7C<GxEPD2_730c_GDEY073D46, GxEPD2_730c_GDEY073D46::HEIGHT / 4> display(GxEPD2_730c_GDEY073D46(/*CS=5*/ 10, /*DC=*/17, /*RST=*/16, /*BUSY=*/4)); // GDEY073D46 800x480 7-color, (N-FPC-001 2021.11.26)
SPIClass hspi(HSPI);
#include <x509_crt_bundle.h>
#include <ArduinoJson.h>

const String DEVICE_NAME = "DEADBEEF";
const int DEVICE_WIDTH = 800;
const int DEVICE_HEIGHT = 480;
const String SERVER_URL = "https://10.0.0.4:8080"; // Replace with your server URL
// Device specific settings
String bearer_token = "ef07dddf-c96b-4648-ac3c-5c82d5e9166f";

const int touchPin = 2; // Replace with the actual touch pin number

bool updateImage();
void drawFull();
void setClock();
String getMacAddress();

void setup()
{
  // put your setup code here, to run once:
  Serial.begin(115200);
  Serial.println();
  Serial.println("setup");
  delay(100);
  hspi.begin(12, 13, 11, 10); // remap hspi for EPD (swap pins)
  display.epd2.selectSPI(hspi, SPISettings(4000000, MSBFIRST, SPI_MODE0));
  Serial.println("Init Display");
  display.init(115200);
  // drawFull();
  // Serial.println("Start Display");
  // display.powerOff();

  // WiFiProvisioner provisioner;

  // provisioner.getConfig().SHOW_INPUT_FIELD = false;
  // provisioner.getConfig().SHOW_RESET_FIELD = true;

  // // Set the success callback
  // provisioner.onSuccess(
  //     [](const char *ssid, const char *password, const char *input) {
  //       Serial.printf("Provisioning successful! Connected to SSID: %s\n", ssid);
  //       if (password) {
  //         Serial.printf("Password: %s\n", password);
  //       }
  //     });

  // // Start provisioning
  // provisioner.startProvisioning();
  Serial.println("Start WiFi");
  WiFi.begin("Vivianite", "elcondorpasa"); // Replace with your WiFi SSID and password
  Serial.println("Connecting to WiFi...");
  while (WiFi.status() != WL_CONNECTED)
  {
    delay(500);
    Serial.print(".");
  }
  setClock();
}

void loop()
{
  // Serial.println(touchRead(touchPin));
  if (touchRead(touchPin) >20000) // Adjust the threshold value as needed
  {
    Serial.println("Touch detected, updating image...");
    updateImage();
  }

  delay(1000); // Small delay to debounce the touch input
}

bool reg()
{
  Serial.println("Registering device...");
  // send POST request to server
  NetworkClientSecure *client = new NetworkClientSecure;
  if (client)
  {
    client->setCACertBundle(x509_crt_bundle, x509_crt_bundle_len);

    {
      // Add a scoping block for HTTPClient https to make sure it is destroyed before NetworkClientSecure *client is
      HTTPClient https;
      Serial.print("[HTTPS] begin...\n");
      if (https.begin(*client, SERVER_URL + "/register")) // Replace with your server URL
      {                                                   // HTTPS
        Serial.print("[HTTPS] POST...\n");
        https.addHeader("Content-Type", "application/json");

        // Create JSON payload
        JsonDocument doc;
        doc["device_name"] = DEVICE_NAME;
        doc["device_id"] = getMacAddress();
        String jsonPayload;
        serializeJson(doc, jsonPayload);

        // Send POST request
        int httpCode = https.POST(jsonPayload);

        // Check response
        if (httpCode > 0)
        {
          Serial.printf("[HTTPS] POST... code: %d\n", httpCode);
          String payload = https.getString();
          // decode JSON response
          JsonDocument responseDoc;
          DeserializationError error = deserializeJson(responseDoc, payload);
          if (error)
          {
            Serial.printf("[HTTPS] JSON parse error: %s\n", error.c_str());
            return false; // JSON parse error
          }
          // Check if the registration was successful

          if (responseDoc["message"] != "Device registered")
          {
            Serial.println("[HTTPS] Registration failed");
            return false; // Registration failed
          }

          // Print the response
          Serial.println("[HTTPS] Registration successful");
          bearer_token = responseDoc["token"].as<String>();
          Serial.println("Bearer token: " + bearer_token);

          return true; // Registration successful
        }
        else
        {
          Serial.printf("[HTTPS] POST... failed, error: %s\n", https.errorToString(httpCode).c_str());
          return false; // Registration failed
        }

        https.end();
      }
      else
      {
        Serial.printf("[HTTPS] Unable to connect\n");
        return false;
      }
    }
  }
  else
  {
    return false;
  }
}

bool updateImage()
{
  Serial.println("Updating image...");

  // send POST request to server
  NetworkClientSecure *client = new NetworkClientSecure;
  if (client)
  {
    client->setCACertBundle(x509_crt_bundle, x509_crt_bundle_len);

    {
      // Add a scoping block for HTTPClient https to make sure it is destroyed before NetworkClientSecure *client is
      HTTPClient https;
      Serial.print("[HTTPS] begin...\n");
      if (https.begin(*client, SERVER_URL + "/dev")) // Replace with your server URL
      {                                              // HTTPS
        Serial.print("[HTTPS] POST...\n");
        https.addHeader("Content-Type", "application/json");
        https.addHeader("Authorization", "Bearer " + bearer_token);

        // Create JSON payload
        JsonDocument doc;
        doc["action"] = "update_image";
        String jsonPayload;
        serializeJson(doc, jsonPayload);

        // Send POST request
        int httpCode = https.POST(jsonPayload);

        // Check response
        if (httpCode > 0)
        {
          Serial.printf("[HTTPS] POST... code: %d\n", httpCode);
          String payload = https.getString();
          // decode JSON response
          JsonDocument responseDoc;
          DeserializationError error = deserializeJson(responseDoc, payload);
          if (error)
          {
            Serial.printf("[HTTPS] JSON parse error: %s\n", error.c_str());
            return false; // JSON parse error
          }
          // Check if the update was successful

          if (responseDoc["message"] != "Image updated")
          {
            Serial.println("[HTTPS] Update failed");
            return false; // Update failed
          }

          // Print the response
          Serial.println("[HTTPS] Update successful");
          // print the json response
          JsonArray images = responseDoc["image"].as<JsonArray>();
          // Create array to store downloaded image data
          uint8_t *downloadedImages[7] = {nullptr};
          int imageCount = 0;

          // Loop through the images array and download up to 7 images
          for (JsonVariant image : images)
          {
            if (imageCount >= 7)
              break; // Limit to 7 images

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
        else
        {
          Serial.printf("[HTTPS] POST... failed, error: %s\n", https.errorToString(httpCode).c_str());
          return false; // Update failed
        }

        https.end();
      }
      else
      {
        Serial.printf("[HTTPS] Unable to connect\n");
        return false;
      }
    }
  }
  else
  {
    return false;
  }
}

uint8_t *downloadImage(String download_url)
{
  Serial.println("Downloading image from: " + download_url);

  NetworkClientSecure *client = new NetworkClientSecure;
  if (!client)
  {
    Serial.println("Failed to allocate NetworkClientSecure");
    return nullptr;
  }

  client->setCACertBundle(x509_crt_bundle, x509_crt_bundle_len);

  {
    HTTPClient https;
    Serial.print("[HTTPS] begin download...\n");
    if (https.begin(*client, SERVER_URL + "/" + download_url))
    { // HTTPS
      https.addHeader("Authorization", "Bearer " + bearer_token);

      int httpCode = https.GET();
      if (httpCode > 0)
      {
        Serial.printf("[HTTPS] GET... code: %d\n", httpCode);

        if (httpCode == HTTP_CODE_OK)
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
                Serial.printf("Downloaded: %d of %d bytes\n", totalRead, contentLength);
              }
              delay(1);
            }

            Serial.println("Download complete");
            https.end();
            delete client;

            // Return the buffer instead of freeing it
            return buffer;
          }
        }
      }
      else
      {
        Serial.printf("[HTTPS] GET... failed, error: %s\n", https.errorToString(httpCode).c_str());
      }
      https.end();
    }
    else
    {
      Serial.println("[HTTPS] Unable to connect for download");
    }
  }

  delete client;
  return nullptr;
}

void drawFull(uint8_t *downloadedImages[])
{
  display.firstPage();
  do
  {
    display.drawBitmap(0, 0, downloadedImages[0], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_BLACK);
    display.drawBitmap(0, 0, downloadedImages[1], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_WHITE);
    display.drawBitmap(0, 0, downloadedImages[2], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_BLUE);
    display.drawBitmap(0, 0, downloadedImages[3], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_GREEN);
    display.drawBitmap(0, 0, downloadedImages[4], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_RED);
    display.drawBitmap(0, 0, downloadedImages[5], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_YELLOW);
    display.drawBitmap(0, 0, downloadedImages[6], DEVICE_WIDTH, DEVICE_HEIGHT, GxEPD_ORANGE);

  } while (display.nextPage()); // Repeat while there are pages left
}

void setClock()
{
  configTime(0, 0, "pool.ntp.org");

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
  WiFi.macAddress(mac);
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