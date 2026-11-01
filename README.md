# the-shield

A Discord bot written in Go that reposts messages with cleaner links and previews, then deletes the originals. By default it changes X post links to translated `fixupx.com` links. In channels you choose, it also turns X posts into embeds with full-size images, sends Pixiv links through phixiv, and reposts other links and attachments.

## What it does

What the bot does depends on whether a channel is turned on with [`/secret`](#choosing-channels). In every channel, it replies `Pong!` to `!ping` and `Ping!` to `!pong`.

**In other channels,** the bot only replaces X post links, including `twitter.com` and fixer sites such as `fxtwitter.com`, `vxtwitter.com` and `fixupx.com`. It reposts the message with each link changed to `fixupx.com`, with `/en` added so the post is translated to English, then deletes the original. Anything after the post ID, such as `?s=20`, is removed.

The repost goes through a webhook, so it shows the sender's server nickname and profile picture. Webhooks can't send replies, so a reply is posted by the bot with `**name:**` in front of the text. The bot does the same if the webhook fails, for example without the **Manage Webhooks** permission. It leaves these alone:

- Links wrapped in `<>`
- Messages with attachments
- Forwarded messages
- Messages from bots

**In channels turned on with `/secret`,** the bot checks each new message in this order and acts on the first match. It ignores its own messages, but not those from other bots.

| Message | What the bot does |
| --- | --- |
| X post link, without video | Sends an embed with the post's author, text and date, plus its images as attachments and any text sent with the link |
| X post link, with video | Reposts the message with the link changed to `cunnyx.com` |
| X post link in a spoiler or `<>` | Reposts the message with the link changed to `cunnyx.com`, keeping the spoiler or `<>` |
| Forward with embeds | Copies the embeds and attaches their images |
| Contains a `tenor.com` link | Leaves it alone |
| Contains any other link (`://`) | Reposts the message, changing `pixiv.net` to `phixiv.net` if present |
| Has attachments | Re-uploads the files in an embed showing the sender's name, avatar and text |
| Anything else | Leaves it alone |

X post links also include `twitter.com` and fixer sites such as `fxtwitter.com` and `vxtwitter.com`. If the bot can't build the embed for a post, it falls back to the `cunnyx.com` repost. Reposted X links also lose anything after the post ID.

Forwarded messages are handled using the forwarded text and files. Forwarded attachments are re-uploaded with their text as a normal message, without the embed.

Other behavior:

- **Originals:** the original message is deleted after a successful repost.
- **Phixiv:** once Discord adds a preview to one of the bot's phixiv links, the bot replaces that message with a copy of the preview and its images attached. Animations keep the phixiv link.
- **Deleting reposts:** the person whose message was reposted can delete the repost by reacting to it with 🧱. Nobody else's 🧱 does anything, but members with **Manage Messages** can still delete it the usual way.

## Requirements

- [Go](https://go.dev/dl/) 1.24.1 or newer
- A bot application from the [Discord Developer Portal](https://discord.com/developers/applications)

## Setup

1. In the Developer Portal, create a bot and copy its token.
2. Under the bot's **Privileged Gateway Intents**, turn on **Message Content Intent**.
3. Invite the bot to your server with the `bot` and `applications.commands` scopes and these permissions: **View Channels**, **Send Messages**, **Embed Links**, **Attach Files**, **Read Message History**, **Manage Messages** and **Manage Webhooks**.
4. Create a file named `token.txt` containing only the token:

   ```
   YOUR_TOKEN
   ```

`token.txt` is listed in `.gitignore`. This repository is public, so keep it that way. If the token is ever exposed, reset it in the Developer Portal.

## Running

```bash
go run .
```

Or build a binary and run that instead:

```bash
go build -o the-shield
```

The bot runs until you press Ctrl+C.

The bot records who each repost belongs to in `repost_senders.txt`, in the current directory, so 🧱 deletes keep working after a restart. The file grows by one short line per repost. Use `-senders-file` to choose a different path.

### Choosing channels

Run `/secret` in a channel to turn full message replacing on there, and run it again to go back to replacing only X post links. The reply is only visible to you, and by default only members with the **Manage Channels** permission can use the command.

The list is saved to `secret_channels.txt` in the current directory, so start the bot from the same folder each time. Use `-channels-file` to choose a different path.

To add a channel without the command:

1. In Discord, turn on **Developer Mode** under **User Settings > Advanced**, then right-click the channel and choose **Copy Channel ID**.
2. Stop the bot. It only reads the file at startup, and using `/secret` overwrites the file with the bot's own list, so edits made while it's running can be lost.
3. Add the ID to the file, in the folder you start the bot from:

   ```bash
   echo CHANNEL_ID >> secret_channels.txt
   ```

4. Start the bot again.

Each ID goes on its own line. To turn a channel off, delete its line while the bot is stopped.

### Token options

The bot looks for `token.txt` in the current directory, then next to the executable. Two flags change that:

| Flag | Default | Description |
| --- | --- | --- |
| `-t` | none | The token itself, used instead of the file |
| `-token-file` | `token.txt` | Path to the file containing the token |

## Known limitations

- **Threads are separate channels.** Turning on a channel doesn't include its threads, so run `/secret` inside a thread to turn it on.
- **Only the first X link gets an embed.** In `/secret` channels, any other links stay in the message text as they were sent.
- **`/secret` channels don't credit the sender on X reposts.** The embed shows the post's author, not who shared it. Reposted links don't show the sender either.
- **Replaced phixiv messages aren't remembered across restarts.** The list of handled messages is kept in memory.

## License

No license has been specified.
