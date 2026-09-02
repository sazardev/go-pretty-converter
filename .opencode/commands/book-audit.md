---
description: "Auditoría total del libro (MDX) sin Chrome: fugas de contenido, ids, TOC, links, componentes, frontmatter, layout y coherencia."
agent: analyst
---

Audita el libro completo de: $ARGUMENTS

Aplica el skill book-audit. Inventario de todos los archivos, tabla de ids `[X.Y.Z]`, y caza exhaustiva de fugas de contenido (huérfanos, duplicados, ids malformados, enlaces muertos, componentes sin registrar, fences desbalanceados, fugas de TOC), integridad de frontmatter, riesgos de layout estático y coherencia editorial. Todo hallazgo con `file:line` y severidad 🔴/🟡/🔵/❓. Termina con el veredicto: ¿el libro está listo para build?
