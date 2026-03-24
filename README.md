# llm-usage

CLI personal para ver el uso de tus suscripciones de Codex/ChatGPT y Claude.

## Instalacion rapida

Si ya tenes acceso al repo privado:

```bash
git clone git@github.com:emanuelarcos/llm-usage.git
cd llm-usage
./install.sh
```

Despues lo usas asi:

```bash
llm-usage
```

## Comandos incluidos

- `llm-usage`: muestra Codex y Claude juntos
- `codex-usage`: muestra solo Codex
- `claude-usage`: muestra solo Claude

## Requisitos

- Python 3
- `codex login` hecho si queres usar Codex
- sesion iniciada en Claude Code si queres usar Claude

## Ejemplos

```bash
llm-usage
codex-usage
claude-usage
```

Para ver JSON crudo:

```bash
llm-usage --json
codex-usage --json
claude-usage --json
```

## Si no te toma el comando

Probablemente `~/.local/bin` no esta en tu `PATH`.

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

## Nota

- `codex-usage` usa un endpoint privado de ChatGPT/Codex
- `claude-usage` usa OAuth local de Claude o endpoints web/OAuth de Anthropic
- estos endpoints pueden cambiar sin aviso
