# the-shield

A Discord bot written in Go that reposts messages with cleaner links and previews, then deletes the originals. By default it changes X post links to translated `fixupx.com` links. In channels you choose, it also turns X posts into embeds with full-size images, sends Pixiv links through phixiv, and reposts other links and attachments. Whoever's message was reposted can delete the repost by reacting with 🧱.

## What it does

What the bot does depends on whether a channel is turned on with [`/secret`](#choosing-channels). In every channel, it replies `Pong!` to `!ping` and `Ping!` to `!pong`.

**In other channels,** the bot only replaces X post links, including `twitter.com` and fixer sites such as `fxtwitter.com`, `vxtwitter.com` and `fixupx.com`. It reposts the message with each link changed to `fixupx.com`, with `/en` added so the post is translated to English, then deletes the original. Anything after the post ID, such as `?s=20`, is removed. It leaves these alone:

- Links wrapped in `<>`
- Messages with attachments
- Forwarded messages
- Messages from bots

The repost goes through a webhook, so it shows the sender's server nickname and profile picture. Webhooks can't send replies, so a reply is posted by the bot with `**name:**` in front of the text. The bot does the same if the webhook fails, for example without the **Manage Webhooks** permission.

**In channels turned on with `/secret`,** the bot checks each new message in this order and acts on the first match. It ignores its own messages, but not those from other bots.

| Message | What the bot does |
| --- | --- |
| X post link, without video | Sends an embed with the post's author, text and date and a clean link to the post, plus its images as attachments and any text sent with the link. Posts in other languages show an English translation when one is available |
| X post link, with video | Reposts the message with the link changed to `cunnyx.com` |
| X post link in a spoiler or `<>` | Reposts the message with the link changed to `cunnyx.com`, keeping the spoiler or `<>` |
| Forward with embeds | Copies the embeds and attaches their images |
| Contains a `tenor.com` link | Leaves it alone |
| Contains any other link (`://`) | Reposts the message, changing `pixiv.net` to `phixiv.net` if present. Artwork links get `/1-9` added to show up to nine images, unless they already pick an image or range |
| Has attachments | Re-uploads the files, with any text as a normal message |
| Anything else | Leaves it alone |

X post links also include `twitter.com` and fixer sites such as `fxtwitter.com` and `vxtwitter.com`. If the bot can't build the embed for a post, it falls back to the `cunnyx.com` repost. Reposted X links also lose anything after the post ID.

Forwarded messages are handled using the forwarded text and files.

Other behavior:

- **Originals:** the original message is deleted after a successful repost. If the repost fails, the original stays.
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

The bot runs until you press Ctrl+C. Start it from the same folder each time, because it keeps its files in the current directory (see [Files and flags](#files-and-flags)). To keep it running in the background, see [Running with systemd](#running-with-systemd).

### Choosing channels

Run `/secret` in a channel to turn full message replacing on there, and run it again to go back to replacing only X post links. The reply is only visible to you, and by default only members with the **Manage Channels** permission can use the command.

Threads follow their parent channel, so turning on a channel also turns on all its threads. To turn on a single thread instead, run `/secret` inside it while the parent is off.

The list is saved to `secret_channels.txt`.

To add a channel without the command:

1. In Discord, turn on **Developer Mode** under **User Settings > Advanced**, then right-click the channel and choose **Copy Channel ID**.
2. Stop the bot. It only reads the file at startup, and using `/secret` overwrites the file with the bot's own list, so edits made while it's running can be lost.
3. Add the ID to the file, in the folder you start the bot from:

   ```bash
   echo CHANNEL_ID >> secret_channels.txt
   ```

4. Start the bot again.

Each ID goes on its own line. To turn a channel off, delete its line while the bot is stopped.

### Files and flags

| File | What it holds | Flag to change the path |
| --- | --- | --- |
| `token.txt` | The bot token. The bot looks in the current directory, then next to the executable. | `-token-file` |
| `secret_channels.txt` | Channels turned on with `/secret` | `-channels-file` |
| `repost_senders.txt` | Who each repost belongs to, so 🧱 deletes keep working after a restart. It grows by one short line per repost. | `-senders-file` |

All three are listed in `.gitignore`. You can also pass the token directly with `-t`, which is used instead of the file.

### Running with systemd

On Linux, systemd can start the bot at boot and restart it if it crashes.

1. Stop any copy of the bot that's already running. Two copies would both repost the same messages.
2. In the repository folder, build the bot and print the folder's full path:

   ```bash
   go build -o the-shield
   ```

   ```bash
   pwd
   ```

3. Create the service file:

   ```bash
   sudo nano /etc/systemd/system/the-shield.service
   ```

4. Paste this in, replacing `YOUR_USER` with the user that owns the folder (`whoami` prints it) and `/path/to/the-shield` with the path from `pwd`:

   ```ini
   [Unit]
   Description=the-shield Discord bot
   After=network-online.target
   Wants=network-online.target

   [Service]
   User=YOUR_USER
   WorkingDirectory=/path/to/the-shield
   ExecStart=/path/to/the-shield/the-shield
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```

   `WorkingDirectory` is the folder where the bot looks for its [files](#files-and-flags), so `token.txt` and any existing `secret_channels.txt` and `repost_senders.txt` need to be there.

5. Load the file, then start the bot now and at every boot:

   ```bash
   sudo systemctl daemon-reload
   ```

   ```bash
   sudo systemctl enable --now the-shield
   ```

6. Check that it's running:

   ```bash
   systemctl status the-shield
   ```

To follow the bot's logs:

```bash
journalctl -u the-shield -f
```

To update the bot, run these in the repository folder:

```bash
git pull
```

```bash
go build -o the-shield
```

```bash
sudo systemctl restart the-shield
```

## Known limitations

- **Only the first X link gets an embed.** In `/secret` channels, any other links stay in the message text as they were sent.
- **`/secret` channels don't show the sender.** Reposts come from the bot with no sender's name. X post embeds show the post's author, not who shared it.
- **Replaced phixiv messages aren't remembered across restarts.** The list of handled messages is kept in memory.
- **🧱 only works on recorded reposts.** Reposts made before the bot started recording senders can't be deleted this way, and neither can its other messages, such as `Pong!`.

## License

No license has been specified.
