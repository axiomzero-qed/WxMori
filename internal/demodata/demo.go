package demodata

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/store"
	"github.com/klauspost/compress/zstd"
)

const SessionID = "wxmori-demo@chatroom"

func Create(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(abs); !os.IsNotExist(err) {
		return "", fmt.Errorf("demo directory must not exist: %s", abs)
	}
	if err = os.MkdirAll(filepath.Join(abs, "data", "contact"), 0700); err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Join(abs, "data", "message"), 0700); err != nil {
		return "", err
	}
	cdb, err := store.Open(filepath.Join(abs, "data", "contact", "contact.db"), false)
	if err != nil {
		return "", err
	}
	_, err = cdb.Exec(`CREATE TABLE contact(username TEXT,nick_name TEXT,remark TEXT,alias TEXT);
INSERT INTO contact VALUES('wxmori-demo@chatroom','微忆 Demo 群 / WxMori Demo','','');
INSERT INTO contact VALUES('wxid_demo_alice','小林 / Alice','Wrong remark','');
INSERT INTO contact VALUES('wxid_demo_bob','小周 / Bob','','');
INSERT INTO contact VALUES('twin-a@chatroom','同名 Demo / Twin Demo','','');
INSERT INTO contact VALUES('twin-b@chatroom','同名 Demo / Twin Demo','','');`)
	cdb.Close()
	if err != nil {
		return "", err
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	base := time.Date(2026, 9, 1, 9, 0, 0, 0, loc).Unix()
	table := fmt.Sprintf("Msg_%x", md5.Sum([]byte(SessionID)))
	for shard := 0; shard < 2; shard++ {
		db, e := store.Open(filepath.Join(abs, "data", "message", fmt.Sprintf("message_%d.db", shard)), false)
		if e != nil {
			return "", e
		}
		_, e = db.Exec(`CREATE TABLE Name2Id(user_name TEXT,is_session INTEGER); CREATE TABLE ` + table + `(local_id INTEGER PRIMARY KEY,server_id INTEGER,local_type INTEGER,sort_seq INTEGER,real_sender_id INTEGER,create_time INTEGER,message_content BLOB,WCDB_CT_message_content INTEGER);`)
		if e != nil {
			db.Close()
			return "", e
		}
		sender := "wxid_demo_alice"
		if shard == 1 {
			sender = "wxid_demo_bob"
		}
		_, e = db.Exec("INSERT INTO Name2Id(rowid,user_name,is_session) VALUES(1,?,0),(2,?,1)", sender, SessionID)
		if e != nil {
			db.Close()
			return "", e
		}
		type row struct {
			server, typ, ts int64
			text            string
			compressed      bool
		}
		rows := []row{}
		if shard == 0 {
			rows = []row{{101, 1, base, "wxid_demo_alice:\n你好，微忆！Hello, WxMori!", false}, {102, 1, base + 60, "wxid_demo_alice:\n收件人: 测试用户\n地址: 虚构街道 1 号\nDemo address, not real data.", false}, {103, (2001 << 32) | 49, base + 120, `<msg><appmsg><title>演示转账 / Demo transfer</title></appmsg></msg>`, true}, {104, 10000, base + 180, `<sysmsg><revokemsg><content>小林撤回了一条消息 / Alice recalled a message</content></revokemsg></sysmsg>`, false}, {0, 1, base + 240, "重复消息 / Repeated message", false}, {0, 1, base + 240, "重复消息 / Repeated message", false}, {105, 1, time.Date(2026, 10, 1, 0, 0, 0, 0, loc).Unix(), "十月边界 / October boundary", false}}
		} else {
			rows = []row{{106, 1, (base + 300) * 1000, "wxid_demo_bob:\n我也来测试。Testing from Bob.", true}, {107, 3, base + 360, "", false}, {108, 1, base + 420, "Meeting time: 10:00\nKeep this line.", false}, {101, 1, base, "wxid_demo_alice:\n你好，微忆！Hello, WxMori!", false}}
		}
		enc, e := zstd.NewWriter(nil)
		if e != nil {
			db.Close()
			return "", e
		}
		for i, r := range rows {
			body := []byte(r.text)
			marker := 0
			if r.compressed {
				body = enc.EncodeAll(body, nil)
				marker = 4
			}
			rsid := 1
			if r.server == 101 && shard == 1 {
				_, e = db.Exec("INSERT OR IGNORE INTO Name2Id(rowid,user_name,is_session) VALUES(3,'wxid_demo_alice',0)")
				rsid = 3
			}
			if e == nil {
				_, e = db.Exec("INSERT INTO "+table+" VALUES(?,?,?,?,?,?,?,?)", i+1, r.server, r.typ, i+1, rsid, r.ts, body, marker)
			}
			if e != nil {
				enc.Close()
				db.Close()
				return "", e
			}
		}
		enc.Close()
		for _, id := range []string{"twin-a@chatroom", "twin-b@chatroom"} {
			t := fmt.Sprintf("Msg_%x", md5.Sum([]byte(id)))
			if _, e = db.Exec("CREATE TABLE " + t + " AS SELECT * FROM " + table + " WHERE 0"); e != nil {
				db.Close()
				return "", e
			}
		}
		db.Close()
	}
	path := filepath.Join(abs, "config.json")
	err = config.Save(path, config.Config{DecryptedDir: "data", OutputDir: "exports", Timezone: "Asia/Shanghai"})
	return path, err
}
