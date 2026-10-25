# Hardware

Hardware facts and assumptions, kept separate from the software. Every claim carries one of these labels:

- **Documented support** — listed by an official project or vendor source.
- **Community reports** — forum posts or tutorials; may be outdated.
- **Not yet verified** — unknown until we check it ourselves.
- **Physically tested and working** — we printed a real page.

As of 2026-10-25 nothing is physically tested. PrintHub's CUPS/IPP integration works against a development CUPS server whose queues print to `/dev/null` ([ipp-and-cups.md](learning/infra/ipp-and-cups.md)); no real printer has been used yet.

## Target topology

```text
Ubuntu server (PrintHub API) ──IPP over Wi-Fi──> Raspberry Pi Zero WH (CUPS) ──USB OTG hub (powered)──> HP M1132 MFP
                                                                                                    └──> HP CP1215
```

Constraint: the Pi sits next to the printers and the Ubuntu server is elsewhere in the house. **Connecting the printers directly to the Ubuntu server over USB is not viable.**

## Raspberry Pi Zero WH

| Item | Finding | Label |
|---|---|---|
| CPU / RAM / network | ARMv6 single core, 512 MB RAM, 2.4 GHz Wi-Fi, one USB data port | Documented support |
| OS | Raspberry Pi OS Lite 32-bit (Debian 13 "trixie", released 2026-10-06) lists "all Raspberry Pi models"; the 64-bit image excludes the original Zero / Zero W | Documented support |
| Package architecture | Raspberry Pi OS 32-bit packages come from the Raspbian archive, built for ARMv6. Plain Debian `armhf` targets ARMv7 and does not run on the Zero — never mix Debian `armhf` repositories or binaries | Documented support |
| Two printers on one port | Requires a USB OTG hub, preferably externally powered | Not yet verified |
| Rasterization speed | Ghostscript on this CPU is the likely bottleneck | Not yet verified |

## HP LaserJet Professional M1132 MFP (monochrome)

| Item | Finding | Label |
|---|---|---|
| HPLIP | Requires HP's proprietary binary plugin; reported as x86-only in the past, failed to install on Raspberry Pi | Community reports |
| foo2zjs | The OpenPrinting foo2zjs model list names **"HP LaserJet Pro M1132s MFP"** under `foo2xqx`. The plain "M1132" is not listed | Documented support (M1132s only) |
| Raspberry Pi tutorial (2014) | foo2zjs built from source, using the **P1102** PPD because no M1132 PPD existed | Community reports |
| Exact model | M1132 vs M1132s are not assumed to be interchangeable. Check the device label and the USB ID (`lsusb`) | Not yet verified |
| Raspbian package | `printer-driver-foo2zjs 20200505dfsg0-5` (armhf, 2026-04) exists; whether it includes the M1132s entry is unknown | Not yet verified |
| Firmware | Whether the P1102 PPD triggers a firmware upload, and whether the M1132 needs one | Not yet verified |

## HP Color LaserJet CP1215 (color)

| Item | Finding | Label |
|---|---|---|
| foo2zjs | Listed under `foo2hp`, "B/W and color" | Documented support |
| On Raspberry Pi | Forum thread inconclusive; the printer was not detected until a powered hub was suggested | Community reports |
| Color profiles (ICM) | Whether downloaded profiles are needed for acceptable color | Not yet verified |

## Fallback if the Pi cannot process jobs

Keep the Pi next to the printers in every option:

1. **Rasterize on Ubuntu, Pi as raw spooler** — CUPS on Ubuntu runs Ghostscript + `foo2xqx` / `foo2hp`, then sends the printer-ready stream to a raw queue on the Pi (CUPS raw queue or `p910nd` on port 9100). The Pi only forwards bytes to USB. Status reporting and Wi-Fi stability must be validated.
2. **Raspberry Pi Zero 2 W** — same form factor, ARMv7/ARMv8 quad core. Cheap upgrade if option 1 is unsatisfactory.
3. **USB/IP** — last resort; reliability over 2.4 GHz Wi-Fi must be proven before relying on it.

## Validation checklist (milestone 13)

The step-by-step setup and a results table live in the runbook: [raspberry-pi-print-bridge.md](runbooks/raspberry-pi-print-bridge.md).

- [ ] Identify exact printer models and USB IDs
- [ ] Both printers enumerate through the hub (`lsusb`)
- [ ] Drivers installed from Raspbian packages or built on the Pi
- [ ] Test page: M1132 monochrome
- [ ] Test page: CP1215 monochrome and color
- [ ] Time to print a 1-page and a 10-page PDF
- [ ] Status reported correctly over IPP (idle, printing, offline)
- [ ] Both printers usable in the same session
- [ ] Survives printer power cycle, Pi reboot and Wi-Fi reconnect

## Sources

- [OpenPrinting/foo2zjs](https://github.com/OpenPrinting/foo2zjs)
- [Raspbian foo2zjs packages](http://archive.raspbian.org/raspbian/pool/main/f/foo2zjs/)
- [M1132 on Raspberry Pi (2014 tutorial)](https://juanvillar.blogspot.com/2014/11/hp-laserjet-pro-m1132-mfp-printer.html)
- [HPLIP ARM plugin question (Launchpad)](https://answers.launchpad.net/hplip/+question/215851)
- [CP1215 on Raspberry Pi (forum)](https://forums.raspberrypi.com/viewtopic.php?t=79676)
- [Raspberry Pi OS downloads](https://www.raspberrypi.com/software/operating-systems/)
