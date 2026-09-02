---
description: "Revisión estricta de código. Detecta el lenguaje y aplica el skill adecuado. Salida: hallazgos one-line por severidad."
agent: reviewer
---

Revisa: $ARGUMENTS

Detecta el lenguaje del target y aplica el skill correspondiente (review-go, review-typescript, review-shell o deep-analysis). Devuelve los hallazgos en una línea por finding con severidad 🔴/🟡/🔵/❓ y `file:line`, agrupados por severidad, con veredicto final.
