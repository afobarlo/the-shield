package main

import (
	//"embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"path"
	"github.com/bwmarrin/discordgo"
	"log"
	"io"
	"net/http"
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
	Token string
)

func init() {
	
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + "")
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}
	
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	
	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages
	
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
		return
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
	if strings.Contains(m.Content, "x.com") {
		messageUrl := strings.Replace(m.Content, "/x.com", "/cunnyx.com", 1)
		vx_url := strings.Replace(m.Content, "x.com", "api.vxtwitter.com", 1)
		resp, err := http.Get(vx_url)
		if err != nil {
			fmt.Println("Error get")
			s.ChannelMessageSend(m.ChannelID, messageUrl)
			err := s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				log.Println("Cannot delete message: %v", err)
			}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error readall")
			s.ChannelMessageSend(m.ChannelID, messageUrl)
			err := s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				log.Println("Cannot delete message: %v", err)
			}
			return
		}
		var vx_data VXTwitter
		err = json.Unmarshal(body, &vx_data)
		if err != nil {
			fmt.Println("Error lectura")
			s.ChannelMessageSend(m.ChannelID, messageUrl)
			err := s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				log.Println("Cannot delete message: %v", err)
			}
			return
		}
		
		if vx_data.CombinedMediaURL != "" {
			
			// 
			var fileList []string
			var fileName string
			var filePath string
			var discordFiles []*discordgo.File
			fileName = path.Base(vx_data.CombinedMediaURL)
			fileList = append(fileList, fileName)
			filePath = "./" + fileName
			resp, err := http.Get(vx_data.CombinedMediaURL)
			if err != nil {
				log.Println(err)
				return
			}
			defer resp.Body.Close()
			
			imgData, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
				return
			}
			
			output, err := os.Create(filePath)
			if err != nil {
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, messageUrl)
				err := s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Println("Cannot delete message: %v", err)
				}
				return
			}
			defer output.Close()
			
			if _, err := output.Write(imgData); err != nil {
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, messageUrl)
				err := s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Println("Cannot delete message: %v", err)
				}
				return
			}
			
			file, err := os.Open(filePath)
			if err != nil {
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, messageUrl)
				err := s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Println("Cannot delete message: %v", err)
				}
				return
			}
			discordFiles = append(discordFiles, &discordgo.File{Name: fileName, Reader: file})
			
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: "<" + m.Content + ">", Files: discordFiles})
			
			err = s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				log.Println("Cannot delete message: %v", err)
			}
			
			for _, fileName := range fileList {
				err := os.Remove(fileName)
				if err != nil {
					log.Println(err)
				}
			}
			//
			
			return
			} else {
			
			if strings.HasSuffix(vx_data.MediaURLs[0], ".mp4") {
				s.ChannelMessageSend(m.ChannelID, messageUrl)
				err := s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Println("Cannot delete message: %v", err)
				}
				} else {
				// 
				var fileList []string
				var fileName string
				var filePath string
				var discordFiles []*discordgo.File
				fileName = path.Base(vx_data.MediaURLs[0])
				fileList = append(fileList, fileName)
				filePath = "./" + fileName
				resp, err := http.Get(vx_data.MediaURLs[0])
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, messageUrl)
					err := s.ChannelMessageDelete(m.ChannelID, m.ID)
					if err != nil {
						log.Println("Cannot delete message: %v", err)
					}
					return
				}
				defer resp.Body.Close()
				
				imgData, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, messageUrl)
					err := s.ChannelMessageDelete(m.ChannelID, m.ID)
					if err != nil {
						log.Println("Cannot delete message: %v", err)
					}
					return
				}
				
				output, err := os.Create(filePath)
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, messageUrl)
					err := s.ChannelMessageDelete(m.ChannelID, m.ID)
					if err != nil {
						log.Println("Cannot delete message: %v", err)
					}
					return
				}
				defer output.Close()
				
				if _, err := output.Write(imgData); err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, messageUrl)
					err := s.ChannelMessageDelete(m.ChannelID, m.ID)
					if err != nil {
						log.Println("Cannot delete message: %v", err)
					}
					return
				}
				
				file, err := os.Open(filePath)
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, messageUrl)
					err := s.ChannelMessageDelete(m.ChannelID, m.ID)
					if err != nil {
						log.Println("Cannot delete message: %v", err)
					}
					return
				}
				discordFiles = append(discordFiles, &discordgo.File{Name: fileName, Reader: file})
				
				s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Content: "<" + m.Content + ">", Files: discordFiles})
				
				err = s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Println("Cannot delete message: %v", err)
				}
				
				for _, fileName := range fileList {
					err := os.Remove(fileName)
					if err != nil {
						log.Println(err)
					}
				}
				//
			}
			
			return
		}
		
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
	
	if m.Attachments != nil {
		if len(m.Attachments) == 0 {
			return
		}
		var fileList []string
		var fileName string
		var filePath string
		var discordFiles []*discordgo.File
		for i := range(len(m.Attachments)) {
			fileName = strings.Split(path.Base(m.Attachments[i].URL), "?")[0]
			fileList = append(fileList, fileName)
			filePath = "./" + fileName
			resp, err := http.Get(m.Attachments[i].URL)
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
	
}
