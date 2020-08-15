// This file is automatically generated. Do not modify it manually.

package main

import (
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

var manifest *model.Manifest

const manifestStr = `
{
  "id": "gather-users",
  "name": "Gather users",
  "description": "This plugin pair two user to chat.",
  "version": "0.1.0",
  "min_server_version": "5.12.0",
  "server": {
    "executables": {
      "linux-amd64": "server/dist/plugin-linux-amd64",
      "darwin-amd64": "server/dist/plugin-darwin-amd64",
      "windows-amd64": "server/dist/plugin-windows-amd64.exe"
    },
    "executable": ""
  },
  "settings_schema": {
    "header": "",
    "footer": "",
    "settings": [
      {
        "key": "Cron",
        "display_name": "Recurrence",
        "type": "dropdown",
        "help_text": "",
        "placeholder": "",
        "default": "@weekly",
        "options": [
          {
            "display_name": "Weekly",
            "value": "@weekly"
          },
          {
            "display_name": "Daily",
            "value": "@daily"
          },
          {
            "display_name": "Monthly",
            "value": "@montly"
          }
        ]
      },
      {
        "key": "InitText",
        "display_name": "Initial text",
        "type": "text",
        "help_text": "",
        "placeholder": "",
        "default": "Let's chat!"
      }
    ]
  }
}
`

func init() {
	manifest = model.ManifestFromJson(strings.NewReader(manifestStr))
}
