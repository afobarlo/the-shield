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
)

// Matches tweet links on x.com, twitter.com and the embed fixer sites.
// Group 1 is the user name, group 2 the tweet ID.
var tweetLink = regexp.MustCompile(`https?://(?:www\.|mobile\.|d\.)?(?:x|twitter|cunnyx|fxtwitter|vxtwitter|fixupx|fixvx|twittpr|girlcockx|stupidpenisx|xcancel)\.com/(\w+)/status/(\d+)\S*`)

// IDs of the bot's phixiv messages that were already rehosted.
var rehostedPhixiv sync.Map

// Matches a phixiv link that isn't wrapped in <>.
var phixivLink = regexp.MustCompile(`(?:^|[^<])https?://(?:www\.)?phixiv\.net/`)

// Matches a link, with or without <> around it. Group 1 is the link itself.
var linkPattern = regexp.MustCompile(`<?(https?://[^\s<>]+)>?`)

func init() {
	
	flag.StringVar(&Token, "t", "", "Bot Token (overrides the token file)")
	flag.StringVar(&TokenFile, "token-file", "token.txt", "File containing the bot token")
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

	// Message events for reposting, reaction events for 🧱 deletes.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions
	
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
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
		var discordFiles []*discordgo.File
		for _, imageUrl := range vx_data.MediaURLs {
			file, err := downloadFile(imageUrl, fileNameFromUrl(imageUrl))
			if err != nil {
				log.Println(err)
				repost(s, m, messageUrl)
				return
			}
			discordFiles = append(discordFiles, file)
		}
		
		_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}, Files: discordFiles})
		if err != nil {
			log.Println(err)
			repost(s, m, messageUrl)
			return
		}
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
		if sent {
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
		s.ChannelMessageSend(m.ChannelID, messageUrl)
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			log.Println("Cannot delete message: %v", err)
		}
		return
	}
	
	if len(m.Attachments) > 0 {
		embed := &discordgo.MessageEmbed{
			Description: m.Content,
			Color:       0x5865F2,
			Author:      &discordgo.MessageEmbedAuthor{Name: m.Author.Username, IconURL: m.Author.AvatarURL("")},
			Timestamp:   m.Timestamp.Format(time.RFC3339),
		}
		// Files are sent as normal attachments, not inside the embed, so Discord shows them at full size
		var discordFiles []*discordgo.File
		for _, attachment := range m.Attachments {
			file, err := downloadFile(attachment.URL, attachment.Filename)
			if err != nil {
				log.Println(err)
				continue
			}
			discordFiles = append(discordFiles, file)
		}
		if len(discordFiles) == 0 {
			return
		}

		message := &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}, Files: discordFiles}
		// Forwards only repost the files, with their text as a normal message
		if isForward {
			message = &discordgo.MessageSend{Content: m.Content, Files: discordFiles}
		}
		_, err := s.ChannelMessageSendComplex(m.ChannelID, message)
		if err != nil {
			log.Println(err)
			return
		}
		deleteOriginal(s, m)
	}
	
}

// repostEmbeds sends a copy of embeds with their images, and the given attachments,
// downloaded and attached at full size. It returns false if there was nothing to send.
func repostEmbeds(s *discordgo.Session, channelID string, content string, attachments []*discordgo.MessageAttachment, sourceEmbeds []*discordgo.MessageEmbed) (bool, error) {
	var discordFiles []*discordgo.File
	// Embed images that point at one of the message's own attachments are only downloaded once
	downloaded := map[string]bool{}
	for _, attachment := range attachments {
		file, err := downloadFile(attachment.URL, attachment.Filename)
		if err != nil {
			log.Println(err)
			continue
		}
		discordFiles = append(discordFiles, file)
		downloaded[stripQuery(attachment.URL)] = true
	}
	var embeds []*discordgo.MessageEmbed
	for _, embed := range sourceEmbeds {
		for _, media := range embedMedia(embed) {
			// Discord allows at most 10 files per message
			if downloaded[stripQuery(media.url)] || len(discordFiles) >= 10 {
				continue
			}
			downloaded[stripQuery(media.url)] = true
			file, err := downloadFile(media.url, fileNameFromUrl(media.url))
			if err != nil && media.proxyUrl != "" {
				// Fall back to Discord's cached copy
				file, err = downloadFile(media.proxyUrl, fileNameFromUrl(media.proxyUrl))
			}
			if err != nil {
				log.Println(err)
				continue
			}
			discordFiles = append(discordFiles, file)
		}
		if embedCopy := copyEmbed(embed); embedCopy != nil {
			embeds = append(embeds, embedCopy)
		}
	}
	if len(embeds) == 0 && len(discordFiles) == 0 {
		return false, nil
	}

	// Links are wrapped in <> so Discord doesn't add its own preview next to the copied embed
	content = linkPattern.ReplaceAllString(content, "<$1>")
	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Content: content, Embeds: embeds, Files: discordFiles})
	if err != nil {
		return false, err
	}
	return true, nil
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
	if sent {
		err := s.ChannelMessageDelete(message.ChannelID, message.ID)
		if err != nil {
			log.Printf("Cannot delete message: %v", err)
		}
	}
}

// messageReactionAdd deletes one of the bot's own messages when someone reacts to it with 🧱.
func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.Emoji.Name != "🧱" || r.UserID == s.State.User.ID {
		return
	}
	message, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		log.Println(err)
		return
	}
	if message.Author == nil || message.Author.ID != s.State.User.ID {
		return
	}
	err = s.ChannelMessageDelete(r.ChannelID, r.MessageID)
	if err != nil {
		log.Printf("Cannot delete message: %v", err)
	}
}

// tweetEmbed builds an embed that mirrors the tweet: author, text, stats and date.
func tweetEmbed(vx_data VXTwitter) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		URL:         vx_data.TweetURL,
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

// repost sends content in place of the original message and deletes the original.
func repost(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	s.ChannelMessageSend(m.ChannelID, content)
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
