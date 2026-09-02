---
name: idea-dev
description: Develop an idea into a validated, scalable plan. Clarifies the idea, checks feasibility against the real codebase, designs flows and data, lists risks and security surface, and outputs milestones. Use when user says "idea", "qué te parece", "develop this idea", "brainstorm", "design", "flujo", or wants to plan a feature.
---

# Idea Development

Turn a raw idea into a concrete, validated, scalable plan grounded in the real codebase. Delegate to the architect agent when appropriate; follow this workflow otherwise.

## Workflow

1. **Clarify (2 sentences max).** Restate the idea: *what* it does, *who* it serves, *why* now. If the idea is vague, restate your best interpretation and list the open questions as assumptions — don't stall on questions.
2. **Anchor to the repo.** Read AGENTS.md, go.mod/package.json. Map the idea to existing modules:
   - `reutiliza <module>` — already exists, extend it
   - `modifica <file>` — touch an existing file
   - `nuevo <package>` — fresh component
   Follow the repo's conventions (naming, error handling, config, tests).
3. **Design the flow.** Data flow with states and error paths:
   `entrada → paso → estado de error → salida`. Include the failure modes at each step.
4. **Risks & scale.** What breaks at 10× data / 10× users / 10× files? What is the single most likely thing to kill the idea, and the cheapest experiment to validate it first?
5. **Security surface.** What untrusted input crosses boundaries? What must be validated/limited?
6. **Plan.** Milestones in dependency order. Each with a Definition of Done and the tests it requires.

## Output format

```markdown
# Idea: <nombre>
## En una frase
## Supuestos
## Arquitectura (reutiliza / modifica / nuevo)
## Flujo de datos (con estados de error)
## Riesgos y escalabilidad
## Superficie de seguridad
## Plan
- [ ] M1 <objetivo> — DoD: <criterio> — tests: <qué>
```

## Rules

- Small composable pieces > monoliths. Everything testable.
- Prefer existing deps over new ones.
- End with the cheapest validation experiment for the biggest risk.
