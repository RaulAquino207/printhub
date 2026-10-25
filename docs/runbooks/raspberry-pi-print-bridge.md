# Raspberry Pi print bridge

Step-by-step setup of the Raspberry Pi Zero WH as the CUPS print bridge next to the two HP printers, and the validation that milestone 13 requires.

> **Status: Not yet verified.** Written on 2026-10-25, before the Pi arrived. No command below has been run on the real hardware. Record every result in [Results](#results) as you go, and update [hardware.md](../hardware.md) only with what was actually observed.

## Target

```text
PrintHub API (Ubuntu server) ──IPP :631──> print-bridge (Pi Zero WH, CUPS) ──USB hub──> HP LaserJet M1132 MFP
                                                                                 └────> HP Color LaserJet CP1215
```

PrintHub talks to the Pi with `PRINTHUB_PRINTING_PROVIDER=cups` and `PRINTHUB_CUPS_URL=http://<pi>:631`. The CUPS **queue names become PrintHub printer IDs**, so choose them once and never rename them: `hp-m1132` and `hp-cp1215`.

## What you need

- Raspberry Pi Zero WH, microSD card (8 GB or more), 5 V / 2.5 A micro-USB power supply.
- Micro-USB OTG adapter + **externally powered** USB hub (the Zero has a single data port and cannot power two printer interfaces reliably).
- The two printers with their USB cables and power (the external voltage transformers mentioned in the project brief — check they are rated correctly before connecting anything).
- A computer with [Raspberry Pi Imager](https://www.raspberrypi.com/software/).

## 1. Flash the OS

In Raspberry Pi Imager:

1. Device: *Raspberry Pi Zero*. OS: **Raspberry Pi OS Lite (32-bit)**. The 64-bit image does not support the Zero / Zero W.
2. Customisation: hostname `print-bridge`, your user, **SSH with your public key** (no password login), Wi-Fi (the Zero W only has 2.4 GHz), locale and timezone `America/Sao_Paulo`.
3. Write the card, boot the Pi, and give it a **DHCP reservation** on the router (as was done for the server).

## 2. First boot

```bash
ssh <user>@print-bridge.local
sudo apt update && sudo apt full-upgrade -y    # slow on the Zero: expect tens of minutes
uname -m                                        # expect armv6l
cat /etc/os-release | grep VERSION_CODENAME     # expect trixie
```

Only use packages from the Raspberry Pi OS repositories: they are built for ARMv6. Never add plain Debian `armhf` repositories or binaries (ARMv7).

## 3. Identify the printers

Connect the powered hub and both printers, power them on:

```bash
lsusb                      # record the vendor:product IDs and exact names
sudo dmesg | grep -i usb | tail -20
```

Write down whether the label and `lsusb` say **M1132** or **M1132s** — the driver choice depends on it.

## 4. Install CUPS and the drivers

```bash
sudo apt install -y cups printer-driver-foo2zjs
sudo usermod -aG lpadmin "$USER"
lpinfo -v                                   # USB device URIs, e.g. usb://HP/LaserJet%20Professional%20M1132%20MFP?serial=…
lpinfo -m | grep -iE 'M1132|P1102|CP1215'   # which PPDs the packaged foo2zjs offers
```

Driver plan per printer (see [hardware.md](../hardware.md) for the sources):

| Printer | First try | If missing |
|---|---|---|
| CP1215 | `foo2hp` PPD for the CP1215 from `printer-driver-foo2zjs` | Build foo2zjs from [OpenPrinting/foo2zjs](https://github.com/OpenPrinting/foo2zjs) (`make && sudo make install && sudo make cups`) |
| M1132 / M1132s | `foo2xqx` PPD for the M1132s, if listed and the device is an M1132s | The P1102 PPD, as in the 2014 tutorial; then the source build. Check whether a firmware upload step (`getweb`) is required |

Do not install HPLIP's proprietary plugin on the Pi: it has been reported unavailable for ARM.

## 5. Create the queues

```bash
sudo lpadmin -p hp-cp1215 -E \
  -v 'usb://HP/Color%20LaserJet%20CP1215?serial=…' \
  -m '<CP1215 PPD from lpinfo -m>' \
  -D 'HP Color LaserJet CP1215' -L 'Ao lado da impressora'

sudo lpadmin -p hp-m1132 -E \
  -v 'usb://HP/LaserJet%20Professional%20M1132%20MFP?serial=…' \
  -m '<M1132 PPD from lpinfo -m>' \
  -D 'HP LaserJet M1132 MFP' -L 'Ao lado da impressora'

lpstat -p -d
```

`-D` becomes the name shown in PrintHub; `-p` (the queue name) becomes the printer ID.

## 6. Local test pages

```bash
lp -d hp-m1132 /usr/share/cups/data/testprint
lp -d hp-cp1215 /usr/share/cups/data/testprint
lp -d hp-cp1215 -o print-color-mode=color some-color.pdf
time lp -d hp-m1132 ten-pages.pdf     # note how long the Pi takes (Ghostscript is the likely bottleneck)
lpstat -W completed -o
```

Look at the paper: a job marked completed by CUPS is not proof.

## 7. Allow the PrintHub server — and only it

Allow remote IPP but restrict it to the Ubuntu server:

```bash
sudo cupsctl --remote-any=no
sudo sed -i 's/^Listen localhost:631/Port 631/' /etc/cups/cupsd.conf
```

In `/etc/cups/cupsd.conf`, inside `<Location />`, allow only the server and local access:

```text
<Location />
  Order allow,deny
  Allow localhost
  Allow 192.168.3.185
</Location>
```

```bash
sudo systemctl restart cups
sudo apt install -y ufw
sudo ufw allow from 192.168.3.0/24 to any port 22 proto tcp
sudo ufw allow from 192.168.3.185 to any port 631 proto tcp
sudo ufw enable
```

PrintHub submits jobs as `requesting-user-name = printhub` without CUPS authentication; the default CUPS policy lets the same user cancel its own jobs. Verify that cancelling from PrintHub works.

## 8. Connect PrintHub

From the Ubuntu server (or locally, pointing at the Pi):

```bash
curl -s http://print-bridge.local:631/printers/ | head    # CUPS answers
PRINTHUB_PRINTING_PROVIDER=cups PRINTHUB_CUPS_URL=http://print-bridge.local:631 \
  moon run api:dev web:dev
```

Do **not** set `PRINTHUB_CUPS_SIMULATED` here: these are the real printers. In the UI both printers must appear without the "Simulada" badge, with the capabilities CUPS reports (color only on the CP1215).

## 9. If the Pi is too slow or a driver will not run

Keep the Pi next to the printers in every option ([hardware.md](../hardware.md#fallback-if-the-pi-cannot-process-jobs)):

1. **Rasterize on the server, raw queues on the Pi**: install CUPS + foo2zjs on Ubuntu, create the driver queues there with `socket://` or `ipp://` URIs pointing at **raw** queues on the Pi (`lpadmin -p hp-m1132-raw -E -v usb://… -m raw`). PrintHub then talks to the server's CUPS.
2. **Raspberry Pi Zero 2 W** (ARMv7/ARMv8, quad core) in the same role.
3. **USB/IP** only as a last resort, after proving it stable over 2.4 GHz Wi-Fi.

## Results

Fill in during milestone 13. Keep "Not yet verified" until there is evidence.

| # | Check | Result | Notes |
|---|---|---|---|
| 1 | Exact models and USB IDs (M1132 vs M1132s) | Not yet verified | |
| 2 | Both printers enumerate through the powered hub (`lsusb`) | Not yet verified | |
| 3 | Driver for the M1132 installed (which PPD) | Not yet verified | |
| 4 | Driver for the CP1215 installed (which PPD) | Not yet verified | |
| 5 | Test page, M1132 monochrome | Not yet verified | |
| 6 | Test page, CP1215 monochrome | Not yet verified | |
| 7 | Test page, CP1215 color (colors acceptable?) | Not yet verified | |
| 8 | Time for a 1-page and a 10-page PDF on each printer | Not yet verified | |
| 9 | PrintHub lists both printers with correct capabilities | Not yet verified | |
| 10 | Print from PrintHub on each printer | Not yet verified | |
| 11 | Cancel from PrintHub while a job is pending | Not yet verified | |
| 12 | Printer powered off → PrintHub shows it offline | Not yet verified | |
| 13 | Pi reboot and Wi-Fi reconnect: printing works afterwards | Not yet verified | |
| 14 | Remote IPP refused from any host other than the server | Not yet verified | |
