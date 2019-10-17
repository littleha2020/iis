package manager

import (
	"regexp"

	"github.com/gin-gonic/gin"
)

var (
	rxCrawler = regexp.MustCompile(`(?i)(bot|googlebot|crawler|spider|robot|crawling)`)
)

type KeyValueOp interface {
	Lock(string)
	Unlock(string)
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}

func IsCrawler(g *gin.Context) bool {
	if rxCrawler.MatchString(g.Request.UserAgent()) {
		return true
	}
	v, _ := g.Cookie("crawler")
	return v == "1"
}

//func (m *Manager) IncrCounter(g *gin.Context, id string) {
//	if IsCrawler(g) {
//		return
//	}
//
//	m.dbCounter.Lock(id)
//	defer m.dbCounter.Unlock(id)
//
//	x, _ := m.counter.m.LoadOrStore(id, &[256]bool{})
//
//	ip := g.MustGet("ip").(net.IP)
//	(*x)[ip[len(ip)-1]] = true
//
//	count := 0
//	m.counter.m.Range(func(k, v interface{}) bool {
//		count++
//		if count > 64 {
//			go m.writeCounterToDB()
//			return false
//		}
//		return true
//	})
//
//	m.counter.k.Reschedule(func() { go m.writeCounterToDB() }, time.Second*10)
//}
//
//func (m *Manager) writeCounterToDB() {
//	m.counter.k.Cancel()
//	count := 0
//
//	m.counter.m.Range(func(k, v interface{}) bool {
//		count++
//		m.dbCounter.Get(
//		return true
//	})
//
//	m.counter.m = sync.Map{}
//	log.Println("[writeCounterToDB] sched:", count)
//}
