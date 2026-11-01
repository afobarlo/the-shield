package main

import (
	//"embed"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"path"
	"path/filepath"
	"github.com/bwmarrin/discordgo"
	"log"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
	"encoding/json"
)

type VXTwitter struct {
	AllSameType      bool     `json:"allSameType,omitempty"`
	Article          any      `json:"article,omitempty"`
	CombinedMediaURL string   `json:"combinedMediaUrl,omitempty"`
	CommunityNote    any      `json:"communityNote,omitempty"`
	ConversationID   string   `json:"conversationID,omitempty"`
	Date             string   `json:"date,omitempty"`
	DateEpoch        int      `json:"date_epoch,omitempty"`
	HasMedia         bool     `json:"hasMedia,omitempty"`
	Hashtags         []string `json:"hashtags,omitempty"`
	Lang             string   `json:"lang,omitempty"`
	Likes            int      `json:"likes,omitempty"`
	MediaURLs        []string `json:"mediaURLs,omitempty"`
	MediaExtended    []struct {
		AltText any `json:"altText,omitempty"`
		Size    struct {
			Height int `json:"height,omitempty"`
			Width  int `json:"width,omitempty"`
		} `json:"size,omitempty"`
		ThumbnailURL string `json:"thumbnail_url,omitempty"`
		Type         string `json:"type,omitempty"`
		URL          string `json:"url,omitempty"`
	} `json:"media_extended,omitempty"`
	PollData            any    `json:"pollData,omitempty"`
	PossiblySensitive   bool   `json:"possibly_sensitive,omitempty"`
	Qrt                 any    `json:"qrt,omitempty"`
	QrtURL              any    `json:"qrtURL,omitempty"`
	Replies             int    `json:"replies,omitempty"`
	Retweets            int    `json:"retweets,omitempty"`
	Text                string `json:"text,omitempty"`
	TweetID             string `json:"tweetID,omitempty"`
	TweetURL            string `json:"tweetURL,omitempty"`
	UserName            string `json:"user_name,omitempty"`
	UserProfileImageURL string `json:"user_profile_image_url,omitempty"`
	UserScreenName      string `json:"user_screen_name,omitempty"`
}

type Phixiv struct {
	ImageProxyUrls  []string  `json:"image_proxy_urls"`
	Title           string    `json:"title"`
	AiGenerated     bool      `json:"ai_generated"`
	Description     string    `json:"description"`
	Tags            []string  `json:"tags"`
	URL             string    `json:"url"`
	AuthorName      string    `json:"author_name"`
	AuthorID        string    `json:"author_id"`
	IsUgoira        bool      `json:"is_ugoira"`
	CreateDate      time.Time `json:"create_date"`
	IllustID        string    `json:"illust_id"`
	ProfileImageURL string    `json:"profile_image_url"`
	Language        string    `json:"language"`
	BookmarkCount   int       `json:"bookmark_count"`
	LikeCount       int       `json:"like_count"`
	CommentCount    int       `json:"comment_count"`
	ViewCount       int       `json:"view_count"`
	XRestrict       int       `json:"x_restrict"`
}

// 
// Variables used for command line parameters
var (
	Token     string
	TokenFile string
	ChannelsFile string
	SendersFile string
)

// Who each repost was made for, by repost message ID. Only that person can delete it with 🧱.
var (
	repostSenders     = map[string]string{}
	repostSendersLock sync.RWMutex
)

// Channels where the bot replaces messages, turned on and off with /secret.
var (
	secretChannels     = map[string]bool{}
	secretChannelsLock sync.RWMutex
)

// The start of a tweet link on x.com, twitter.com and the embed fixer sites, up to the user name.
const tweetSite = `https?://(?:www\.|mobile\.|d\.)?(?:x|twitter|cunnyx|fxtwitter|vxtwitter|fixupx|fixvx|twittpr|girlcockx|stupidpenisx|xcancel)\.com/`

// The end of a tweet link: the user name, the tweet ID and any trailing path or identifiers,
// stopping before characters that close spoilers or <> around the link.
const tweetEnd = `(\w+)/status/(\d+)(?:[/?][^\s|,>]*)?`

// Matches a tweet link. Group 1 is the user name, group 2 the tweet ID.
var tweetLink = regexp.MustCompile(tweetSite + tweetEnd)

// The bot's webhook for each channel, used to repost under the sender's name and picture.
var channelWebhooks sync.Map

// The parent of each thread, and each other channel as its own parent, so they're only looked up once.
var parentChannels sync.Map

// IDs of the bot's phixiv messages that were already rehosted.
var rehostedPhixiv sync.Map

// Matches a phixiv link that isn't wrapped in <>.
var phixivLink = regexp.MustCompile(`(?:^|[^<])https?://(?:www\.)?phixiv\.net/`)

// Matches a phixiv artwork link. Group 1 is the image number or range, if it has one.
var phixivArtwork = regexp.MustCompile(`phixiv\.net/(?:[\w-]+/)?artworks/\d+(/[\d-]+)?/?`)

// Matches a link, with or without <> around it. Group 1 is the link itself.
var linkPattern = regexp.MustCompile(`<?(https?://[^\s<>]+)>?`)

// Matches a tweet link that isn't wrapped in <>. Group 1 is the character before
// the link, group 2 the user name and group 3 the tweet ID.
var plainTweetLink = regexp.MustCompile(`(^|[^<])` + tweetSite + tweetEnd)

func init() {
	
	flag.StringVar(&Token, "t", "", "Bot Token (overrides the token file)")
	flag.StringVar(&TokenFile, "token-file", "token.txt", "File containing the bot token")
	flag.StringVar(&ChannelsFile, "channels-file", "secret_channels.txt", "File where the channels turned on with /secret are saved")
	flag.StringVar(&SendersFile, "senders-file", "repost_senders.txt", "File where the sender of each repost is saved")
	flag.Parse()
}

func main() {

	if Token == "" {
		token, err := readToken(TokenFile)
		if err != nil {
			fmt.Println("error reading bot token,", err)
			return
		}
		Token = token
	}

	err := loadSecretChannels(ChannelsFile)
	if err != nil {
		fmt.Println("error reading secret channels,", err)
		return
	}

	err = loadRepostSenders(SendersFile)
	if err != nil {
		fmt.Println("error reading repost senders,", err)
		return
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}
	
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageUpdate)
	dg.AddHandler(messageReactionAdd)
	dg.AddHandler(interactionCreate)

	// Message events for reposting, reaction events for 🧱 deletes.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions
	
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	
	// Register /secret. Channels that are already turned on keep working if this fails.
	_, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", secretCommand())
	if err != nil {
		log.Printf("Cannot register /secret: %v", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	
	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	//var output string
	
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		// Except phixiv reposts that already come with their embed
		rehostPhixiv(s, m.Message)
		return
	}

	// Forwarded messages carry their text and files in a snapshot, so treat them like a normal message
	isForward := false
	var forwardedEmbeds []*discordgo.MessageEmbed
	if m.MessageReference != nil && m.MessageReference.Type == discordgo.MessageReferenceTypeForward && len(m.MessageSnapshots) > 0 {
		if forwarded := m.MessageSnapshots[0].Message; forwarded != nil {
			isForward = true
			m.Content = forwarded.Content
			m.Attachments = forwarded.Attachments
			forwardedEmbeds = forwarded.Embeds
		}
	}
	// If the message is "ping" reply with "Pong!"
	if m.Content == "!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
		return
	}
	
	// If the message is "pong" reply with "Ping!"
	if m.Content == "!pong" {
		s.ChannelMessageSend(m.ChannelID, "Ping!")
		return
	}

	// Outside the channels turned on with /secret, only tweet links are replaced
	if !channelIsSecret(s, m.ChannelID) {
		if !isForward {
			replaceTweetLinks(s, m)
		}
		return
	}
	/*
		if strings.Contains(m.Content, "pixiv.net") {
		phixivUrl := strings.Replace(m.Content, "pixiv.net", "phixiv.net", 1)
		
		messageUrl := strings.Replace(m.Content, "pixiv.net/en/artworks/", "pixiv.net/ajax/illust/", 1)
		//messageUrl = messageUrl + "?language=en"
		resp, err := http.Get(messageUrl)
		if err != nil {
		fmt.Println("Error get")
		return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
		fmt.Println("Error readall")
		return
		}
		
		var vx_data Phixiv
		err = json.Unmarshal(body, &vx_data)
		if err != nil {
		fmt.Println("Error lectura")
		return
		}
		
		if strings.HasSuffix(vx_data.ImageProxyUrls[0], ".mp4") {
		s.ChannelMessageSend(m.ChannelID, phixivUrl)
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
		log.Println("Cannot delete message: %v", err)
		}
		return
		} else {
		var fileList []string
		var fileName string
		var filePath string
		var discordFiles []*discordgo.File
		for i := range len(vx_data.ImageProxyUrls) {
		fileName = path.Base(vx_data.ImageProxyUrls[i])
		fileList = append(fileList, fileName)
		filePath = "./" + fileName
		resp, err := http.Get(vx_data.ImageProxyUrls[i])
		if err != nil {
		log.Println(err)
		continue
		}
		defer resp.Body.Close()
		
		imgData, err := io.ReadAll(resp.Body)
		if err != nil {
		log.Println(err)
		continue
		}
		
		output, err := os.Create(filePath)
		if err != nil {
		log.Println(err)
		continue
		}
		defer output.Close()
		
		if _, err := output.Write(imgData); err != nil {
		log.Println(err)
		continue
		}
		
		file, err := os.Open(filePath)
		if err != nil {
		log.Println(err)
		continue
		}
		discordFiles = append(discordFiles, &discordgo.File{Name: fileName, Reader: file})
		if len(discordFiles) > 9 {
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: m.Content, Files: discordFiles})
		discordFiles = []*discordgo.File{}
		for _, fileName := range fileList {
		err := os.Remove(fileName)
		if err != nil {
		log.Println(err)
		}
		}
		fileList = []string{}
		}
		
		}
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: m.Content, Files: discordFiles})
		
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
		log.Println("Cannot delete message: %v", err)
		}
		
		for _, fileName := range fileList {
		err := os.Remove(fileName)
		if err != nil {
		log.Println(err)
		}
		}
		
		}
		return
		} 
	*/
	// x.com, twitter.com and fixer links (cunnyx, fxtwitter, vxtwitter...) all get the same embed
	if match := tweetLink.FindStringSubmatch(m.Content); match != nil {
		tweetPath := match[1] + "/status/" + match[2]
		messageUrl := strings.Replace(m.Content, match[0], "https://cunnyx.com/"+tweetPath, 1)

		// Links in a spoiler or <> stay links, since an embed would show what the sender hid
		before, _, _ := strings.Cut(m.Content, match[0])
		if strings.Count(before, "||")%2 == 1 || strings.HasSuffix(before, "<") {
			repost(s, m, messageUrl)
			return
		}

		vx_data, err := fetchTweet("https://api.vxtwitter.com/" + tweetPath)
		if err != nil {
			log.Println(err)
			repost(s, m, messageUrl)
			return
		}

		// Embeds can't play video, so videos keep using the cunnyx link
		if hasVideo(vx_data) {
			repost(s, m, messageUrl)
			return
		}
		
		embed := tweetEmbed(vx_data)
		// Images are sent as normal attachments, not inside the embed, so Discord shows them at full size
		var downloads []fileDownload
		for _, imageUrl := range vx_data.MediaURLs {
			downloads = append(downloads, fileDownload{url: imageUrl, name: fileNameFromUrl(imageUrl)})
		}
		discordFiles, failed := downloadAll(downloads)
		if failed > 0 {
			repost(s, m, messageUrl)
			return
		}
		
		// Keep the text around the link, which the embed replaces
		text := strings.Replace(m.Content, match[0], "", 1)
		sent, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: strings.TrimSpace(text), Embeds: []*discordgo.MessageEmbed{embed}, Files: discordFiles})
		if err != nil {
			log.Println(err)
			repost(s, m, messageUrl)
			return
		}
		recordSender(sent.ID, m.Author.ID)
		deleteOriginal(s, m)
		return
	}

	// Forwarded embeds of any kind (bot embeds, link previews...) are reposted like tweets:
	// the embed's text is copied and its images are downloaded and attached at full size
	if len(forwardedEmbeds) > 0 {
		sent, err := repostEmbeds(s, m.ChannelID, m.Content, m.Attachments, forwardedEmbeds)
		if err != nil {
			log.Println(err)
			return
		}
		if sent != nil {
			recordSender(sent.ID, m.Author.ID)
			deleteOriginal(s, m)
		}
		return
	}

	if strings.Contains(m.Content, "://") {
		messageUrl := strings.Replace(m.Content, "/x.com", "/cunnyx.com", 1)
		messageUrl = strings.Replace(messageUrl, "pixiv.net", "phixiv.net", 1)
		if strings.Contains(m.Content, "tenor.com") {
			return
		}
		// Ask for the first nine images, unless the link already picks some
		messageUrl = phixivArtwork.ReplaceAllStringFunc(messageUrl, func(link string) string {
			if phixivArtwork.FindStringSubmatch(link)[1] != "" {
				return link
			}
			return strings.TrimSuffix(link, "/") + "/1-9"
		})
		repost(s, m, messageUrl)
		return
	}

	if len(m.Attachments) > 0 {
		var downloads []fileDownload
		for _, attachment := range m.Attachments {
			downloads = append(downloads, fileDownload{url: attachment.URL, name: attachment.Filename})
		}
		discordFiles, _ := downloadAll(downloads)
		if len(discordFiles) == 0 {
			return
		}

		sent, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: m.Content, Files: discordFiles})
		if err != nil {
			log.Println(err)
			return
		}
		recordSender(sent.ID, m.Author.ID)
		deleteOriginal(s, m)
	}
	
}

// repostEmbeds sends a copy of embeds with their images, and the given attachments,
// downloaded and attached at full size. It returns the new message, or nil if there was nothing to send.
func repostEmbeds(s *discordgo.Session, channelID string, content string, attachments []*discordgo.MessageAttachment, sourceEmbeds []*discordgo.MessageEmbed) (*discordgo.Message, error) {
	var downloads []fileDownload
	// Embed images that point at one of the message's own attachments are only downloaded once
	downloaded := map[string]bool{}
	for _, attachment := range attachments {
		downloads = append(downloads, fileDownload{url: attachment.URL, name: attachment.Filename})
		downloaded[stripQuery(attachment.URL)] = true
	}
	var embeds []*discordgo.MessageEmbed
	for _, embed := range sourceEmbeds {
		for _, media := range embedMedia(embed) {
			// Discord allows at most 10 files per message
			if downloaded[stripQuery(media.url)] || len(downloads) >= 10 {
				continue
			}
			downloaded[stripQuery(media.url)] = true
			downloads = append(downloads, fileDownload{url: media.url, proxyUrl: media.proxyUrl, name: fileNameFromUrl(media.url)})
		}
		if embedCopy := copyEmbed(embed); embedCopy != nil {
			embeds = append(embeds, embedCopy)
		}
	}
	discordFiles, _ := downloadAll(downloads)
	if len(embeds) == 0 && len(discordFiles) == 0 {
		return nil, nil
	}

	// Links are wrapped in <> so Discord doesn't add its own preview next to the copied embed
	content = linkPattern.ReplaceAllString(content, "<$1>")
	return s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Content: content, Embeds: embeds, Files: discordFiles})
}

// messageUpdate catches the moment Discord adds a link's embed to one of the bot's messages.
func messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if len(m.Embeds) == 0 {
		return
	}
	message := m.Message
	// Embed-only updates can leave out the author and text, so fetch the full message
	if message.Author == nil {
		var err error
		message, err = s.ChannelMessage(m.ChannelID, m.ID)
		if err != nil {
			log.Println(err)
			return
		}
	}
	rehostPhixiv(s, message)
}

// rehostPhixiv replaces one of the bot's own phixiv links, once Discord has added the link's
// embed, with a copy of that embed and its images attached, like a tweet repost.
func rehostPhixiv(s *discordgo.Session, message *discordgo.Message) {
	// The rehosted copy wraps its link in <>, so only a plain phixiv link still needs rehosting
	if message.Author == nil || message.Author.ID != s.State.User.ID || len(message.Embeds) == 0 || !phixivLink.MatchString(message.Content) {
		return
	}
	for _, embed := range message.Embeds {
		// Animations (ugoira) keep the phixiv link, since embeds can't play video
		if embed.Video != nil && embed.Video.URL != "" {
			return
		}
	}
	// Discord can send the same embed more than once, so each message is only rehosted once
	if _, done := rehostedPhixiv.LoadOrStore(message.ID, true); done {
		return
	}
	sent, err := repostEmbeds(s, message.ChannelID, message.Content, message.Attachments, message.Embeds)
	if err != nil {
		log.Println(err)
		return
	}
	if sent != nil {
		recordSender(sent.ID, repostSender(message.ID))
		err := s.ChannelMessageDelete(message.ChannelID, message.ID)
		if err != nil {
			log.Printf("Cannot delete message: %v", err)
		}
	}
}

// messageReactionAdd deletes a repost when the person it was made for reacts to it with 🧱.
func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.Emoji.Name != "🧱" || r.UserID != repostSender(r.MessageID) {
		return
	}
	err := s.ChannelMessageDelete(r.ChannelID, r.MessageID)
	if err != nil {
		log.Printf("Cannot delete message: %v", err)
	}
}

// tweetEmbed builds an embed that mirrors the tweet: author, text, stats and date.
func tweetEmbed(vx_data VXTwitter) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		// The URL only shows up as a link on the title. It's built here so it has no tracking
		Title:       "View post on X",
		URL:         "https://x.com/" + vx_data.UserScreenName + "/status/" + vx_data.TweetID,
		Description: truncate(vx_data.Text, 4096),
		Color:       0x1DA1F2,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    truncate(fmt.Sprintf("%s (@%s)", vx_data.UserName, vx_data.UserScreenName), 256),
			URL:     "https://x.com/" + vx_data.UserScreenName,
			IconURL: vx_data.UserProfileImageURL,
		},
		/* Fields: []*discordgo.MessageEmbedField{
			{Name: "💬 Replies", Value: fmt.Sprint(vx_data.Replies), Inline: true},
			{Name: "🔁 Retweets", Value: fmt.Sprint(vx_data.Retweets), Inline: true},
			{Name: "❤️ Likes", Value: fmt.Sprint(vx_data.Likes), Inline: true},
		}, */
	}
	if vx_data.DateEpoch != 0 {
		embed.Timestamp = time.Unix(int64(vx_data.DateEpoch), 0).Format(time.RFC3339)
	}
	return embed
}

// replaceTweetLinks reposts a message with its tweet links changed to fixupx, which translates
// the posts to English, and deletes the original. Bot messages are left alone, and so are
// messages with attachments, since this repost wouldn't include them.
func replaceTweetLinks(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || len(m.Attachments) > 0 {
		return
	}
	content := plainTweetLink.ReplaceAllString(m.Content, "${1}https://fixupx.com/$2/status/$3/en")
	if content == m.Content {
		return
	}
	// The original is only deleted if the repost went through
	err := repostAsSender(s, m, content)
	if err != nil {
		log.Println(err)
		return
	}
	deleteOriginal(s, m)
}

// repostAsSender sends content through a webhook showing the sender's name and picture.
// Webhooks can't reply to messages, so replies, and anything the webhook can't send,
// go through the bot with the sender's name in the text.
func repostAsSender(s *discordgo.Session, m *discordgo.MessageCreate, content string) error {
	name := m.Author.DisplayName()
	if m.Member != nil && m.Member.Nick != "" {
		name = m.Member.Nick
	}
	if m.MessageReference == nil {
		webhook, threadID, err := channelWebhook(s, m.ChannelID)
		if err == nil {
			// The original message already pinged anyone it mentions, so this repost doesn't
			var sent *discordgo.Message
			sent, err = s.WebhookThreadExecute(webhook.ID, webhook.Token, true, threadID, &discordgo.WebhookParams{
				Content:         content,
				Username:        name,
				AvatarURL:       m.Author.AvatarURL(""),
				AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
			})
			if err == nil {
				recordSender(sent.ID, m.Author.ID)
				return nil
			}
			// In case the webhook was deleted, look it up again next time
			channelWebhooks.Delete(webhook.ChannelID)
		}
		log.Println(err)
	}
	sent, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: "**" + name + ":** " + content, Reference: m.MessageReference})
	if err != nil {
		return err
	}
	recordSender(sent.ID, m.Author.ID)
	return nil
}

// parentChannel returns a thread's parent channel, or the channel itself if it isn't a thread.
// That never changes, so each channel is only looked up once.
func parentChannel(s *discordgo.Session, channelID string) (string, error) {
	if parentID, ok := parentChannels.Load(channelID); ok {
		return parentID.(string), nil
	}
	channel, err := s.Channel(channelID)
	if err != nil {
		return "", err
	}
	parentID := channelID
	if channel.IsThread() {
		parentID = channel.ParentID
	}
	parentChannels.Store(channelID, parentID)
	return parentID, nil
}

// channelWebhook returns the bot's webhook for a channel, creating it if needed. Threads use
// their parent channel's webhook, so it also returns the thread's ID, or "" outside a thread.
func channelWebhook(s *discordgo.Session, channelID string) (*discordgo.Webhook, string, error) {
	parentID, err := parentChannel(s, channelID)
	if err != nil {
		return nil, "", err
	}
	threadID := ""
	if parentID != channelID {
		threadID = channelID
	}
	if webhook, ok := channelWebhooks.Load(parentID); ok {
		return webhook.(*discordgo.Webhook), threadID, nil
	}
	webhooks, err := s.ChannelWebhooks(parentID)
	if err != nil {
		return nil, "", err
	}
	for _, webhook := range webhooks {
		if webhook.ApplicationID == s.State.User.ID && webhook.Token != "" {
			channelWebhooks.Store(parentID, webhook)
			return webhook, threadID, nil
		}
	}
	webhook, err := s.WebhookCreate(parentID, "the-shield", "")
	if err != nil {
		return nil, "", err
	}
	channelWebhooks.Store(parentID, webhook)
	return webhook, threadID, nil
}

// repostSender returns who a repost was made for, or "" if the message isn't a known repost.
func repostSender(messageID string) string {
	repostSendersLock.RLock()
	defer repostSendersLock.RUnlock()
	return repostSenders[messageID]
}

// recordSender remembers who a repost was made for and adds it to the file, so that
// person can still delete it after a restart.
func recordSender(messageID string, senderID string) {
	if senderID == "" {
		return
	}
	repostSendersLock.Lock()
	defer repostSendersLock.Unlock()
	repostSenders[messageID] = senderID

	file, err := os.OpenFile(SendersFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, messageID, senderID)
	if err != nil {
		log.Println(err)
	}
}

// loadRepostSenders reads the saved reposts, one "messageID senderID" per line. A missing file means there are none.
func loadRepostSenders(sendersFile string) error {
	data, err := os.ReadFile(sendersFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	fields := strings.Fields(string(data))
	for i := 0; i+1 < len(fields); i += 2 {
		repostSenders[fields[i]] = fields[i+1]
	}
	return nil
}

// secretCommand describes /secret, which only members who can manage channels see by default.
func secretCommand() *discordgo.ApplicationCommand {
	var permissions int64 = discordgo.PermissionManageChannels
	dmPermission := false
	return &discordgo.ApplicationCommand{
		Name:                     "secret",
		Description:              "Turn full message replacing on or off in this channel",
		DefaultMemberPermissions: &permissions,
		DMPermission:             &dmPermission,
	}
}

// interactionCreate handles /secret, turning message replacing on or off in the channel it's used in.
func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand || i.ApplicationCommandData().Name != "secret" {
		return
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: secretReply(s, i.ChannelID), Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		log.Println(err)
	}
}

// secretReply flips a channel and returns what to tell the user. A thread whose parent
// is turned on can't be changed on its own, since it follows the parent.
func secretReply(s *discordgo.Session, channelID string) string {
	parentID, err := parentChannel(s, channelID)
	if err != nil {
		log.Println(err)
		return "Couldn't check this channel, so nothing changed."
	}
	if parentID != channelID && isSecretChannel(parentID) {
		return "This thread follows its parent channel, which is turned on. Run /secret there to change it."
	}
	enabled, err := toggleSecretChannel(channelID)
	if err != nil {
		log.Println(err)
		return "Couldn't save the change, so nothing changed."
	}
	if enabled {
		return "The bot will now replace messages in this channel."
	}
	return "The bot will now only replace X and Twitter links in this channel."
}

// channelIsSecret tells whether a channel, or the parent of a thread, is turned on with /secret.
func channelIsSecret(s *discordgo.Session, channelID string) bool {
	if isSecretChannel(channelID) {
		return true
	}
	parentID, err := parentChannel(s, channelID)
	if err != nil {
		log.Println(err)
		return false
	}
	return isSecretChannel(parentID)
}

func isSecretChannel(channelID string) bool {
	secretChannelsLock.RLock()
	defer secretChannelsLock.RUnlock()
	return secretChannels[channelID]
}

// toggleSecretChannel flips a channel and saves the list. It returns whether the channel is now on.
func toggleSecretChannel(channelID string) (bool, error) {
	secretChannelsLock.Lock()
	defer secretChannelsLock.Unlock()
	enabled := !secretChannels[channelID]
	secretChannels[channelID] = enabled

	var channelIDs []string
	for id, on := range secretChannels {
		if on {
			channelIDs = append(channelIDs, id)
		}
	}
	err := os.WriteFile(ChannelsFile, []byte(strings.Join(channelIDs, "\n")+"\n"), 0644)
	if err != nil {
		secretChannels[channelID] = !enabled
		return !enabled, err
	}
	return enabled, nil
}

// loadSecretChannels reads the saved channel IDs, one per line. A missing file means no channels are on.
func loadSecretChannels(channelsFile string) error {
	data, err := os.ReadFile(channelsFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, channelID := range strings.Fields(string(data)) {
		secretChannels[channelID] = true
	}
	return nil
}

// readToken reads the bot token from tokenFile, looking in the current
// directory first and then next to the executable.
func readToken(tokenFile string) (string, error) {
	data, err := os.ReadFile(tokenFile)
	if err != nil && !filepath.IsAbs(tokenFile) {
		if exe, exeErr := os.Executable(); exeErr == nil {
			data, err = os.ReadFile(filepath.Join(filepath.Dir(exe), tokenFile))
		}
	}
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	token = strings.TrimPrefix(token, "Bot ")
	if token == "" {
		return "", fmt.Errorf("%s is empty", tokenFile)
	}
	return token, nil
}

func hasVideo(vx_data VXTwitter) bool {
	for _, media := range vx_data.MediaExtended {
		if media.Type == "video" || media.Type == "gif" {
			return true
		}
	}
	for _, mediaUrl := range vx_data.MediaURLs {
		if strings.Contains(mediaUrl, ".mp4") {
			return true
		}
	}
	return false
}

func fetchTweet(vx_url string) (VXTwitter, error) {
	var vx_data VXTwitter
	resp, err := http.Get(vx_url)
	if err != nil {
		return vx_data, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return vx_data, fmt.Errorf("vxtwitter %s: %s", vx_url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return vx_data, err
	}
	err = json.Unmarshal(body, &vx_data)
	return vx_data, err
}

// downloadFile fetches fileUrl into memory so it can be re-uploaded with the embed.
func downloadFile(fileUrl string, name string) (*discordgo.File, error) {
	resp, err := http.Get(fileUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", fileUrl, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	// Discord only previews files whose name has an extension
	if path.Ext(name) == "" {
		name += extensionFor(contentType)
	}
	return &discordgo.File{Name: name, ContentType: contentType, Reader: bytes.NewReader(data)}, nil
}

// fileDownload is a file to fetch with downloadAll. proxyUrl is Discord's cached copy, if there is one.
type fileDownload struct {
	url      string
	proxyUrl string
	name     string
}

// downloadAll fetches all the files at the same time. It returns the ones that worked,
// in the original order, and how many failed.
func downloadAll(downloads []fileDownload) ([]*discordgo.File, int) {
	results := make([]*discordgo.File, len(downloads))
	var wg sync.WaitGroup
	for i, download := range downloads {
		wg.Add(1)
		go func(i int, download fileDownload) {
			defer wg.Done()
			file, err := downloadFile(download.url, download.name)
			if err != nil && download.proxyUrl != "" {
				// Fall back to Discord's cached copy
				file, err = downloadFile(download.proxyUrl, fileNameFromUrl(download.proxyUrl))
			}
			if err != nil {
				log.Println(err)
				return
			}
			results[i] = file
		}(i, download)
	}
	wg.Wait()

	var files []*discordgo.File
	for _, file := range results {
		if file != nil {
			files = append(files, file)
		}
	}
	return files, len(downloads) - len(files)
}

type embedMediaUrl struct {
	url      string
	proxyUrl string
}

// embedMedia lists the images of an embed to download and attach. Link previews keep
// their picture in the thumbnail, and GIF previews (Tenor, Giphy) in the video.
func embedMedia(embed *discordgo.MessageEmbed) []embedMediaUrl {
	var media []embedMediaUrl
	if embed.Image != nil && embed.Image.URL != "" {
		media = append(media, embedMediaUrl{embed.Image.URL, embed.Image.ProxyURL})
	}
	if embed.Type == discordgo.EmbedTypeGifv && embed.Video != nil && embed.Video.URL != "" {
		media = append(media, embedMediaUrl{embed.Video.URL, ""})
	} else if embed.Type != discordgo.EmbedTypeRich && embed.Thumbnail != nil && embed.Thumbnail.URL != "" {
		media = append(media, embedMediaUrl{embed.Thumbnail.URL, embed.Thumbnail.ProxyURL})
	}
	return media
}

// copyEmbed copies the parts of a received embed that a bot can send, leaving out the
// images, which are attached as files. It returns nil if nothing is left, like the extra
// image-only embeds of a gallery or a preview of a plain image link.
func copyEmbed(embed *discordgo.MessageEmbed) *discordgo.MessageEmbed {
	embedCopy := &discordgo.MessageEmbed{
		URL:         embed.URL,
		Title:       embed.Title,
		Description: embed.Description,
		Timestamp:   embed.Timestamp,
		Color:       embed.Color,
		Fields:      embed.Fields,
	}
	if embed.Author != nil {
		embedCopy.Author = &discordgo.MessageEmbedAuthor{Name: embed.Author.Name, URL: embed.Author.URL, IconURL: embed.Author.IconURL}
	}
	if embed.Footer != nil {
		embedCopy.Footer = &discordgo.MessageEmbedFooter{Text: embed.Footer.Text, IconURL: embed.Footer.IconURL}
	}
	// Bots can't send the site name of a link preview, so show it in the footer instead
	if embed.Provider != nil && embed.Provider.Name != "" && embedCopy.Footer == nil {
		embedCopy.Footer = &discordgo.MessageEmbedFooter{Text: embed.Provider.Name}
	}
	// Small thumbnails (logos, avatars) stay on bot embeds; on link previews the thumbnail is the attached picture
	if embed.Type == discordgo.EmbedTypeRich && embed.Thumbnail != nil {
		embedCopy.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: embed.Thumbnail.URL}
	}
	if embedCopy.Title == "" && embedCopy.Description == "" && embedCopy.Author == nil && embedCopy.Footer == nil && len(embedCopy.Fields) == 0 && embedCopy.Thumbnail == nil {
		return nil
	}
	return embedCopy
}

func extensionFor(contentType string) string {
	switch strings.TrimSpace(strings.Split(contentType, ";")[0]) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	}
	return ""
}

func fileNameFromUrl(fileUrl string) string {
	u, err := url.Parse(fileUrl)
	if err != nil {
		return "image.jpg"
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" {
		return "image"
	}
	return name
}

func stripQuery(fileUrl string) string {
	return strings.Split(fileUrl, "?")[0]
}

// repost sends content in place of the original message and deletes the original if that worked.
func repost(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	sent, err := s.ChannelMessageSend(m.ChannelID, content)
	if err != nil {
		log.Println(err)
		return
	}
	recordSender(sent.ID, m.Author.ID)
	deleteOriginal(s, m)
}

func deleteOriginal(s *discordgo.Session, m *discordgo.MessageCreate) {
	err := s.ChannelMessageDelete(m.ChannelID, m.ID)
	if err != nil {
		log.Printf("Cannot delete message: %v", err)
	}
}

func truncate(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-1]) + "…"
}
