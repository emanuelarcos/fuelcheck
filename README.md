# llm-usage

CLI para ver el uso de tus suscripciones de Codex/ChatGPT y Claude desde una sola carpeta.

## Incluye

- `codex-usage`
- `claude-usage`
- `llm-usage`

## Requisitos

- Python 3
- sesion iniciada con `codex login` para Codex
- sesion iniciada con Claude Code o credenciales exportadas para Claude

## Instalar

```bash
./install.sh
```

## Uso

```bash
llm-usage
codex-usage
claude-usage
```

JSON crudo:

```bash
llm-usage --json
codex-usage --json
claude-usage --json
```

## Nota

- `codex-usage` usa un endpoint privado de ChatGPT/Codex.
- `claude-usage` usa OAuth local de Claude o endpoints web/OAuth de Anthropic.
- Estos endpoints pueden cambiar sin aviso.
