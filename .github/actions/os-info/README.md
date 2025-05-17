# OS Info GitHub Action

This action detects and outputs information about the current operating system and architecture directly to stdout.

## Information Provided

The action outputs the following information directly to the console:

| Information | Description |
|-------------|-------------|
| OS | The operating system (Linux, macOS, Windows) |
| Architecture | The architecture (amd64, arm64, etc.) |
| OS Version | The OS version or distribution |

## Example Usage

```yaml
jobs:
  example-job:
    runs-on: ubuntu-latest
    steps:
      - name: Get OS Info
        uses: launchrctl/launchr/.github/actions/os-info@main
        # The action will output OS information directly to the console
        # No need to capture or use outputs in subsequent steps

      - uses: actions/checkout@v4
```
