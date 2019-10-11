package config

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"io/ioutil"
	"net"
	"regexp"

	"gopkg.in/yaml.v2"
)

var Cfg = struct {
	CacheSize    int64    `yaml:"CacheSize"`
	Key          string   `yaml:"Key"`
	TokenTTL     int64    `yaml:"TokenTTL"`
	IDTokenTTL   int64    `yaml:"IDTokenTTL"`
	MaxContent   int64    `yaml:"MaxContent"`
	MinContent   int64    `yaml:"MinContent"`
	AdminName    string   `yaml:"AdminName"`
	PostsPerPage int      `yaml:"PostsPerPage"`
	Tags         []string `yaml:"Tags"`
	Domain       string   `yaml:"Domain"`
	InboxSize    int      `yaml:"InboxSize"`
	IPBlacklist  []string `yaml:"IPBlacklist"`
	Cooldown     int      `yaml:"Cooldown"`
	NeedID       bool     `yaml:"NeedID"`

	// inited after config being read
	Blk               cipher.Block
	KeyBytes          []byte
	IPBlacklistParsed []*net.IPNet
	TagsMap           map[string]bool
	PublicString      string
	PrivateString     string
}{
	CacheSize:    1,
	TokenTTL:     1,
	IDTokenTTL:   600,
	Key:          "0123456789abcdef",
	AdminName:    "zzz",
	MaxContent:   4096,
	MinContent:   8,
	PostsPerPage: 30,
	Tags:         []string{},
	InboxSize:    100,
	Cooldown:     10,
}

func MustLoad() {
	buf, err := ioutil.ReadFile("config.yml")
	if err != nil {
		panic(err)
	}

	if err := yaml.Unmarshal(buf, &Cfg); err != nil {
		panic(err)
	}

	Cfg.Blk, _ = aes.NewCipher([]byte(Cfg.Key))
	Cfg.KeyBytes = []byte(Cfg.Key)
	Cfg.TagsMap = map[string]bool{}

	for _, tag := range Cfg.Tags {
		Cfg.TagsMap[tag] = true
	}

	for _, addr := range Cfg.IPBlacklist {
		_, subnet, _ := net.ParseCIDR(addr)
		Cfg.IPBlacklistParsed = append(Cfg.IPBlacklistParsed, subnet)
	}

	buf, _ = json.MarshalIndent(Cfg, "<li>", "    ")
	Cfg.PrivateString = "<li>" + string(buf)
	buf = regexp.MustCompile(`(?i)".*(token|key|admin).+`).ReplaceAllFunc(buf, func(in []byte) []byte {
		return bytes.Repeat([]byte("\u2588"), len(in)/2+1)
	})
	Cfg.PublicString = "<li>" + string(buf)
}
