# Mattermost Gather Users

Mattermost plugin to pair two random user to chat. The plugin tries to avoid users you have already talked to.

## Installation

Download the latest version from [releases](https://github.com/juanfran/mattermost-gather-users/releases).

Go to **System Console > Plugins > Management** upload and enable the plugin.

## Settings 

- **Recurrence** - daily, weekly or monthly meetings.
- **Initial text** - The text that will be send to the users when is time to chat.
- **Start chats on sign in** - If this is activated when the user type '/gather-plugin on' the plugin try to find a meeting instead of waiting to the next one.

## Usage

In any channel you can use the following command. By default you are not going to participate in any meeting until you type `/gather-plugin on`.

- `/gather-plugin on` - You are available to meet, you have to wait until the the plugin assign you a partner to talk.
- `/gather-plugin off` - You don't want to participate in the next recurring meetings.
