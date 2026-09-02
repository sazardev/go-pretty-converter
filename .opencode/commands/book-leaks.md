---
description: "Caza rápida de fugas en el libro (MDX): contenido perdido, duplicados, ids rotos, enlaces muertos, componentes sin registrar."
agent: analyst
---

Caza fugas en el libro: $ARGUMENTS

Modo rápido del skill book-audit. SOLO busca fugas de contenido, en este orden de prioridad:
1. Duplicados y malformaciones de id `[X.Y.Z]`
2. Ficheros huérfanos / contenido que no entra al documento final
3. Enlaces y `depends_on` hacia ids inexistentes
4. Componentes custom usados sin registrar
5. Fences de código desbalanceados que tragan contenido

Ignora coherencia editorial y estilo. Salida: lista plana de fugas con `file:line`, cada una en una línea, agrupadas por prioridad. Veredicto final en una línea.
