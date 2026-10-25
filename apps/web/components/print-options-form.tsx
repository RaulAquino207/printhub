"use client"

import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
  FieldTitle,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import type { PrintOptions } from "@/lib/jobs"
import type {
  ColorMode,
  Orientation,
  PrinterCapabilities,
} from "@/lib/printers"

export const colorModeLabels: Record<ColorMode, string> = {
  color: "Colorida",
  monochrome: "Preto e branco",
}

export const orientationLabels: Record<Orientation, string> = {
  portrait: "Retrato",
  landscape: "Paisagem",
}

export const paperSizeLabels: Record<string, string> = {
  A4: "A4",
  Letter: "Carta",
  Legal: "Ofício",
}

/** Sensible starting options for a printer: one copy, black and white, A4, portrait. */
export function defaultOptions(c: PrinterCapabilities): PrintOptions {
  const pick = <T,>(preferred: T, available: T[]) =>
    available.includes(preferred) ? preferred : available[0]
  return {
    copies: 1,
    colorMode: pick<ColorMode>("monochrome", c.colorModes),
    paperSize: pick("A4", c.paperSizes),
    orientation: pick<Orientation>("portrait", c.orientations),
    duplex: false,
    pageRanges: "",
  }
}

/**
 * Print options for the selected printer. Only what the printer supports is
 * offered: a monochrome printer never shows a color choice.
 */
export function PrintOptionsForm({
  capabilities: c,
  value,
  onChange,
  errors,
  disabled,
}: {
  capabilities: PrinterCapabilities
  value: PrintOptions
  onChange: (value: PrintOptions) => void
  errors: Record<string, string>
  disabled?: boolean
}) {
  const set = <K extends keyof PrintOptions>(key: K, v: PrintOptions[K]) =>
    onChange({ ...value, [key]: v })

  return (
    <FieldGroup className="gap-5">
      <Field data-invalid={Boolean(errors.copies)}>
        <FieldLabel htmlFor="copies">Cópias</FieldLabel>
        <Input
          id="copies"
          type="number"
          inputMode="numeric"
          min={1}
          max={c.maxCopies}
          value={Number.isNaN(value.copies) ? "" : value.copies}
          onChange={(e) => set("copies", e.target.valueAsNumber)}
          aria-invalid={Boolean(errors.copies)}
          disabled={disabled}
          className="w-28"
        />
        <FieldDescription>Até {c.maxCopies}.</FieldDescription>
        <FieldError>{errors.copies}</FieldError>
      </Field>

      {c.colorModes.length > 1 ? (
        <FieldSet>
          <FieldLegend variant="label">Cor</FieldLegend>
          <RadioGroup
            value={value.colorMode}
            onValueChange={(v) => set("colorMode", v as ColorMode)}
            disabled={disabled}
            className="flex gap-6"
          >
            {c.colorModes.map((mode) => (
              <Field key={mode} orientation="horizontal" className="w-auto">
                <RadioGroupItem
                  value={mode}
                  id={`color-${mode}`}
                  className="border-muted-foreground"
                />
                <FieldLabel htmlFor={`color-${mode}`} className="font-normal">
                  {colorModeLabels[mode]}
                </FieldLabel>
              </Field>
            ))}
          </RadioGroup>
          <FieldError>{errors.colorMode}</FieldError>
        </FieldSet>
      ) : (
        <Field>
          <FieldTitle>Cor</FieldTitle>
          <FieldDescription>
            {colorModeLabels[c.colorModes[0]]} (única opção desta impressora)
          </FieldDescription>
        </Field>
      )}

      <Field data-invalid={Boolean(errors.paperSize)}>
        <FieldLabel htmlFor="paper-size">Papel</FieldLabel>
        <Select
          items={c.paperSizes.map((size) => ({
            value: size,
            label: paperSizeLabels[size] ?? size,
          }))}
          value={value.paperSize}
          onValueChange={(v) => set("paperSize", v as string)}
          disabled={disabled}
        >
          <SelectTrigger id="paper-size" className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {c.paperSizes.map((size) => (
              <SelectItem key={size} value={size}>
                {paperSizeLabels[size] ?? size}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldError>{errors.paperSize}</FieldError>
      </Field>

      {c.orientations.length > 1 && (
        <FieldSet>
          <FieldLegend variant="label">Orientação</FieldLegend>
          <RadioGroup
            value={value.orientation}
            onValueChange={(v) => set("orientation", v as Orientation)}
            disabled={disabled}
            className="flex gap-6"
          >
            {c.orientations.map((o) => (
              <Field key={o} orientation="horizontal" className="w-auto">
                <RadioGroupItem
                  value={o}
                  id={`orientation-${o}`}
                  className="border-muted-foreground"
                />
                <FieldLabel
                  htmlFor={`orientation-${o}`}
                  className="font-normal"
                >
                  {orientationLabels[o]}
                </FieldLabel>
              </Field>
            ))}
          </RadioGroup>
          <FieldError>{errors.orientation}</FieldError>
        </FieldSet>
      )}

      {c.duplex && (
        <Field orientation="horizontal">
          <Switch
            id="duplex"
            checked={value.duplex}
            onCheckedChange={(checked) => set("duplex", checked)}
            disabled={disabled}
          />
          <FieldLabel htmlFor="duplex" className="font-normal">
            Frente e verso
          </FieldLabel>
        </Field>
      )}

      {c.pageRanges && (
        <Field data-invalid={Boolean(errors.pageRanges)}>
          <FieldLabel htmlFor="page-ranges">Páginas</FieldLabel>
          <Input
            id="page-ranges"
            value={value.pageRanges}
            onChange={(e) => set("pageRanges", e.target.value)}
            aria-invalid={Boolean(errors.pageRanges)}
            disabled={disabled}
            autoComplete="off"
            className="w-48"
          />
          <FieldDescription>
            Deixe em branco para imprimir todas. Exemplo: 1-3, 5
          </FieldDescription>
          <FieldError>{errors.pageRanges}</FieldError>
        </Field>
      )}
    </FieldGroup>
  )
}
