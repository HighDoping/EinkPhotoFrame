# E-ink Photo Frame

Work in Progress.

This project involves creating a digital photo frame using an e-ink display.

The photo frame will fetch images from the server at predefined intervals and display them on the e-ink screen.

## Features

- Fetch images from a server
- Display images on an e-ink display
- Dithering and scaling on the server.

## Requirements

- E-ink display (7-color 7.3-inch GDEY073D46)
- ESP32-S3 (PSRAM needed for image buffering)
- WiFi connection
- Self-hosted image server

## Firmware build (PlatformIO)

The firmware is an Arduino-framework PlatformIO project for an ESP32-S3 N16R8
(16 MB QIO flash, 8 MB OPI PSRAM). Build it with:

```sh
pio run -e esp32-s3-n16r8
```

Before building, create the deployment-specific `x509_crt_bundle.h`. The
firmware uses this header to validate HTTPS connections, so it is not tracked
in the repository. With no arguments, the helper downloads Espressif's current
Mozilla-derived root bundle:

```sh
python3 arduino/certs/make_cert_bundle.py
```

To use a local CA file instead:

```sh
python3 arduino/certs/make_cert_bundle.py --ca-file /path/to/root-ca.pem
```

Or download a CA file from another HTTPS location and create the bundle:

```sh
python3 arduino/certs/make_cert_bundle.py --ca-url https://example.com/root-ca.pem
```

The downloaded CA is saved to `arduino/certs/downloaded_ca.pem`; the generated
header is saved to `arduino/firmware/x509_crt_bundle.h`. The script requires
the Python `cryptography` package, as does the existing ESP32 bundle generator.
