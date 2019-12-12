package manager

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/coyove/iis/cmd/ch/config"
	"github.com/coyove/iis/cmd/ch/ident"
	"github.com/coyove/iis/cmd/ch/mv"
)

func (m *Manager) GetUser(id string) (*mv.User, error) {
	p, err := m.db.Get("u/" + id)
	if err != nil {
		return nil, err
	}
	return mv.UnmarshalUser(p)
}

func (m *Manager) GetUserByToken(tok string) (*mv.User, error) {
	if tok == "" {
		return nil, fmt.Errorf("invalid token")
	}

	x, err := base64.StdEncoding.DecodeString(tok)
	if err != nil {
		return nil, err
	}

	for i := len(x) - 16; i >= 0; i -= 8 {
		config.Cfg.Blk.Decrypt(x[i:], x[i:])
	}

	parts := bytes.SplitN(x, []byte("\x00"), 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	session, id := parts[0], parts[1]
	u, err := m.GetUser(string(id))
	if err != nil {
		return nil, err
	}

	if u.Session != string(session) {
		return nil, fmt.Errorf("invalid token session")
	}
	return u, nil
}

func (m *Manager) IsBanned(id string) bool {
	u, err := m.GetUser(id)
	if err != nil {
		return true
	}
	return u.Banned
}

func (m *Manager) SetUser(u *mv.User) error {
	if u.ID == "" {
		return nil
	}
	return m.db.Set("u/"+u.ID, u.Marshal())
}

func (m *Manager) LockUserID(id string) {
	m.db.Lock(id)
}

func (m *Manager) UnlockUserID(id string) {
	m.db.Unlock(id)
}

func (m *Manager) UpdateUser(id string, cb func(u *mv.User) error) error {
	m.db.Lock(id)
	defer m.db.Unlock(id)
	return m.UpdateUser_unlock(id, cb)
}

func (m *Manager) UpdateUser_unlock(id string, cb func(u *mv.User) error) error {
	u, err := m.GetUser(id)
	if err != nil {
		return err
	}
	if err := cb(u); err != nil {
		return err
	}
	return m.SetUser(u)
}

func (m *Manager) MentionUser(a *mv.Article, id string) error {
	if m.IsBlocking(id, a.Author) {
		return fmt.Errorf("author blocked")
	}

	if err := m.insertArticle(ident.NewID(ident.IDTagInbox).SetTag(id).String(), &mv.Article{
		ID:  ident.NewGeneralID().String(),
		Cmd: mv.CmdMention,
		Extras: map[string]string{
			"from":       a.Author,
			"article_id": a.ID,
		},
		CreateTime: time.Now(),
	}, false); err != nil {
		return err
	}
	return m.UpdateUser(id, func(u *mv.User) error {
		u.Unread++
		return nil
	})
}

func (m *Manager) FollowUser_unlock(from, to string, following bool) (E error) {
	u, err := m.GetUser(from)
	if err != nil {
		return err
	}

	root := u.FollowingChain
	if u.FollowingChain == "" {
		a := &mv.Article{
			ID:         ident.NewGeneralID().String(),
			Cmd:        mv.CmdFollow,
			CreateTime: time.Now(),
		}
		if err := m.db.Set(a.ID, a.Marshal()); err != nil {
			return err
		}
		if err := m.UpdateUser_unlock(from, func(u *mv.User) error {
			u.FollowingChain = a.ID
			return nil
		}); err != nil {
			return err
		}
		root = a.ID
	}

	followID := makeFollowID(from, to)

	defer func() {
		if E != nil {
			return
		}

		go func() {
			m.UpdateUser(to, func(u *mv.User) error {
				if following {
					u.Followers++
				} else {
					dec0(&u.Followers)
				}
				return nil
			})
			m.UpdateUser(from, func(u *mv.User) error {
				if following {
					u.Followings++
				} else {
					dec0(&u.Followings)
				}
				return nil
			})
		}()
	}()

	if a, _ := m.GetArticle(followID); a != nil {
		state := strconv.FormatBool(following)
		if a.Extras["follow"] == state {
			return nil
		}
		a.Extras["follow"] = state
		return m.db.Set(a.ID, a.Marshal())
	}

	if err := m.insertArticle(root, &mv.Article{
		ID:  followID,
		Cmd: mv.CmdFollow,
		Extras: map[string]string{
			"to":     to,
			"follow": strconv.FormatBool(following),
		},
		CreateTime: time.Now(),
	}, false); err != nil {
		return err
	}

	return nil
}

func (m *Manager) BlockUser_unlock(from, to string, blocking bool) (E error) {
	u, err := m.GetUser(from)
	if err != nil {
		return err
	}

	root := u.BlockingChain
	if u.BlockingChain == "" {
		a := &mv.Article{
			ID:         ident.NewGeneralID().String(),
			Cmd:        mv.CmdBlock,
			CreateTime: time.Now(),
		}
		if err := m.db.Set(a.ID, a.Marshal()); err != nil {
			return err
		}
		if err := m.UpdateUser_unlock(from, func(u *mv.User) error {
			u.BlockingChain = a.ID
			return nil
		}); err != nil {
			return err
		}
		root = a.ID
	}

	followID := makeBlockID(from, to)

	if a, _ := m.GetArticle(followID); a != nil {
		state := strconv.FormatBool(blocking)
		if a.Extras["block"] == state {
			return nil
		}
		a.Extras["block"] = state
		return m.db.Set(a.ID, a.Marshal())
	}

	if err := m.insertArticle(root, &mv.Article{
		ID:  followID,
		Cmd: mv.CmdBlock,
		Extras: map[string]string{
			"to":    to,
			"block": strconv.FormatBool(blocking),
		},
		CreateTime: time.Now(),
	}, false); err != nil {
		return err
	}

	return nil
}

type FollowingState struct {
	ID       string
	Time     time.Time
	Followed bool
	Blocked  bool
}

func (m *Manager) GetFollowingList(getBlockList bool, u *mv.User, cursor string, n int) ([]FollowingState, string) {
	chain := func() string {
		if getBlockList {
			return u.BlockingChain
		}
		return u.FollowingChain
	}()

	if chain == "" {
		if cursor != "" {
			if u, _ := m.GetUser(lastElemInCompID(cursor)); u != nil {
				return []FollowingState{{ID: u.ID, Time: u.Signup}}, ""
			}
		}
		return nil, ""
	}

	if cursor == "" {
		a, err := m.GetArticle(chain)
		if err != nil {
			log.Println("[GetFollowingList] Failed to get chain", u, "[", chain, "]")
			return nil, ""
		}
		cursor = a.NextID
	}

	res := []FollowingState{}
	start := time.Now()
	startCursor := cursor

	for len(res) < n && cursor != "" {
		if time.Since(start).Seconds() > 0.2 {
			log.Println("[GetFollowingList] Break out slow walk", u, "[", cursor, "]")
			break
		}

		a, err := m.GetArticle(cursor)
		if err != nil {
			if cursor == startCursor {
				if u, _ := m.GetUser(lastElemInCompID(cursor)); u != nil {
					res = append(res, FollowingState{ID: u.ID, Time: u.Signup})
				}
			}
			log.Println("[GetFollowingList]", cursor, err)
			break
		}

		if a.Extras["follow"] == "true" {
			res = append(res, FollowingState{ID: a.Extras["to"], Time: a.CreateTime, Followed: true})
		} else if a.Extras["block"] == "true" {
			res = append(res, FollowingState{ID: a.Extras["to"], Time: a.CreateTime, Blocked: true})
		} else if cursor == startCursor {
			res = append(res, FollowingState{ID: a.Extras["to"], Time: a.CreateTime})
		}

		cursor = a.NextID
	}

	return res, cursor
}

func (m *Manager) IsFollowing(from, to string) bool {
	p, _ := m.GetArticle(makeFollowID(from, to))
	return p != nil && p.Extras["follow"] == "true"
}

func (m *Manager) IsBlocking(from, to string) bool {
	p, _ := m.GetArticle(makeBlockID(from, to))
	return p != nil && p.Extras["block"] == "true"
}
