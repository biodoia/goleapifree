# Code Page 437 Fonts

Per un'esperienza visiva ottimale, puoi scaricare e installare font CP437 autentici.

## Font Consigliati

### IBM VGA Fonts
The Ultimate Oldschool PC Font Pack:
- URL: https://int10h.org/oldschool-pc-fonts/
- Font consigliati:
  - **IBM VGA 8x16** (classico)
  - **Perfect DOS VGA 437**
  - **Px437 IBM VGA9**

### Installazione

1. Scarica il font pack
2. Estrai i file .ttf
3. Copia in questa directory:
   ```bash
   cp /path/to/PxPlus_IBM_VGA9.ttf ./PxPlus_IBM_VGA9.ttf
   ```
4. Aggiorna `cp437.css`:
   ```css
   @font-face {
       font-family: 'IBM VGA';
       src: url('/static/fonts/PxPlus_IBM_VGA9.ttf') format('truetype');
       font-weight: normal;
       font-style: normal;
   }
   ```

## Alternative Web-Safe

Se non vuoi installare font personalizzati, questi font web-safe offrono un aspetto simile:

1. **Consolas** (Windows)
2. **Menlo** (macOS)
3. **Monaco** (macOS)
4. **Courier New** (universale)
5. **monospace** (fallback generico)

Il CSS attuale usa già questi come fallback.

## Font CP437 via CDN

Puoi anche usare CDN per caricare font retrò:

```html
<!-- VT323 (Google Fonts - stile terminale) -->
<link href="https://fonts.googleapis.com/css2?family=VT323&display=swap" rel="stylesheet">

<!-- Press Start 2P (Google Fonts - stile pixel) -->
<link href="https://fonts.googleapis.com/css2?family=Press+Start+2P&display=swap" rel="stylesheet">
```

Aggiorna `cp437.css`:
```css
body {
    font-family: 'VT323', 'Courier New', monospace;
}
```

## Font ASCII Art

Per ASCII art ottimale, usa font monospace con:
- Larghezza carattere fissa
- Altezza linea uniforme
- Glifi completi per box-drawing

Caratteri CP437 importanti:
```
╔═╗╚╝║─│┌┐└┘├┤┬┴┼
█▀▄░▒▓■□▪▫
```

Verifica che il font scelto supporti questi caratteri.
