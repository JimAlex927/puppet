# Puppet Plugin Protocol

Puppet supports two plugin runtimes:

- `exec`: starts one process for each node execution. This is best for small, stateless tools.
- `daemon`: starts a long-running local process, or connects to an existing HTTP URL. This is best for plugins that keep SDK clients, caches, models, or other expensive state.

## Layout

Plugins are discovered from `PUPPET_PLUGIN_DIR`, defaulting to `data/plugins`. Each plugin is a directory with a `plugin.json` manifest:

```json
{
  "id": "example.exec-greeting",
  "name": "Exec Greeting",
  "version": "0.1.0",
  "runtime": "exec",
  "entry": "pwsh",
  "args": ["-NoLogo", "-NoProfile", "-File", "plugin.ps1"],
  "env": {
    "EXAMPLE_VALUE": "hello"
  },
  "nodes": [
    {
      "type": "plugin.execGreeting",
      "name": "Exec Greeting",
      "category": "plugin",
      "description": "Return a greeting from an exec plugin.",
      "supportedOS": ["linux", "darwin", "windows"],
      "fields": [
        { "name": "name", "label": "Name", "type": "input", "required": true }
      ]
    }
  ]
}
```

`entry` can be an absolute path, a file inside the plugin directory, or a command available on `PATH`. `args` are placed before Puppet's command argument.

## Exec Runtime

Puppet runs:

```text
<entry> <args...> execute
```

The plugin receives an `ExecuteRequest` JSON document on stdin and must write one `ExecuteResponse` JSON document to stdout.

## Daemon Runtime

For a managed local daemon, Puppet runs:

```text
<entry> <args...> serve --addr 127.0.0.1:<port>
```

The daemon must expose:

- `GET /health`: returns any 2xx response when ready.
- `POST /execute`: accepts `ExecuteRequest` JSON and returns `ExecuteResponse` JSON.

For an external daemon, set `url` in `plugin.json` instead of `entry`; Puppet will call `<url>/execute` and will not start a process.

## ExecuteRequest

```json
{
  "nodeType": "plugin.execGreeting",
  "params": { "name": "Ada" },
  "workspace": "C:/path/to/workspace",
  "taskRunId": 123,
  "nodeRunId": 456
}
```

## ExecuteResponse

```json
{
  "output": {
    "message": "Hello, Ada"
  },
  "logs": [
    { "stream": "stdout", "content": "greeting generated" }
  ]
}
```

If `error` is non-empty, the node fails but `output` is still recorded:

```json
{
  "output": {},
  "error": "remote service rejected the request"
}
```

Plugin nodes appear in the frontend under the `plugin` category.
