package ssdb

import (
	log "github.com/thinkboy/log4go"
	"fmt"
	"time"
)
var (
	count           int32
	lastBinlogType  string
	datetypeMetrics map[string]int32
	binlogCmdMetrics map[string]int32
)

type SSDBSalve struct {
	id 		 string
	c    *SSDBClient
	from string
	auth string
	status uint32
	connectRetry uint32
	cmdsQueue *chan []string
}


func NewSSDBSalve(from string, fromAuth string, cmdsQueue *chan []string) (*SSDBSalve, error) {
	server := &SSDBSalve{id: fmt.Sprintf("id-%s-%d", from, time.Now().Unix()),
		from: from,
		auth: fromAuth,
		cmdsQueue: cmdsQueue,
	}
	return server, nil

}


func (s *SSDBSalve) Start() (err error) {

	s.status = SALVESTATUS_DISCONNECTED;
	datetypeMetrics = make(map[string]int32, 1000)
	datetypeMetrics[string(DATATYPE_SYNCLOG)] = 0
	datetypeMetrics[string(DATATYPE_KV)] = 0
	datetypeMetrics[string(DATATYPE_HASH)] = 0
	datetypeMetrics[string(DATATYPE_HSIZE)] = 0
	datetypeMetrics[string(DATATYPE_ZSET)] = 0
	datetypeMetrics[string(DATATYPE_ZSCORE)] = 0
	datetypeMetrics[string(DATATYPE_ZSIZE)] = 0
	datetypeMetrics[string(DATATYPE_QUEUE)] = 0
	datetypeMetrics[string(DATATYPE_QSIZE)] = 0

	binlogCmdMetrics = make(map[string]int32, 1000)
	binlogCmdMetrics[string(BINLOGCOMMAND_NONE)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_KSET)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_KDEL)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_HSET)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_HDEL)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_ZSET)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_ZDEL)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_QPUSH_BACK)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_QPUSH_FRONT)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_QPOP_BACK)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_QPOP_FRONT)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_QSET)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_BEGIN)] = 0
	binlogCmdMetrics[string(BINLOGCOMMAND_END)] = 0

	reconnect := true
	for {
		if reconnect {
			s.status = SALVESTATUS_DISCONNECTED
			reconnect = false;
			if s.c != nil {
				s.c.Close()
			}
			s.c = nil
			time.Sleep(time.Second * 2)
		}
		if s.c == nil {
			if err = s.connectToMaster();err != nil {
				time.Sleep(time.Second * 100)
				continue
			}
		}

		for {
			if resp, err := s.c.RecvBinlog();err == nil {
				s.handleRecv(resp)
				continue
			}
			return err
		}
	}
}


func (s *SSDBSalve) handleRecv(binlog *Binlog) (err error) {
	if len(binlog.body[0]) > 0{
		datetypeMetrics[string(binlog.body[0][:1])]++
	}
	binlogCmdMetrics[string(binlog.cmdtype)]++
	count++
	if count % 500000 == 1 {
		log.Info("count is %d %d", s.status, count)
		log.Info("cmd: %v", binlog.cmd)
		log.Info("datetypeMetrics[%d %d %d %d %d %d %d %d %d]",
			datetypeMetrics[string(DATATYPE_SYNCLOG)],
			datetypeMetrics[string(DATATYPE_KV)],
			datetypeMetrics[string(DATATYPE_HASH)],
			datetypeMetrics[string(DATATYPE_HSIZE)],
			datetypeMetrics[string(DATATYPE_ZSET)],
			datetypeMetrics[string(DATATYPE_ZSCORE)],
			datetypeMetrics[string(DATATYPE_ZSIZE)],
			datetypeMetrics[string(DATATYPE_QUEUE)],
			datetypeMetrics[string(DATATYPE_QSIZE)])

		log.Info("binlogCmdMetrics[%d %d %d %d %d %d %d %d %d %d %d %d %d %d]",
			binlogCmdMetrics[string(BINLOGCOMMAND_NONE)],
			binlogCmdMetrics[string(BINLOGCOMMAND_KSET)],
			binlogCmdMetrics[string(BINLOGCOMMAND_KDEL)],
			binlogCmdMetrics[string(BINLOGCOMMAND_HSET)],
			binlogCmdMetrics[string(BINLOGCOMMAND_HDEL)],
			binlogCmdMetrics[string(BINLOGCOMMAND_ZSET)],
			binlogCmdMetrics[string(BINLOGCOMMAND_ZDEL)],
			binlogCmdMetrics[string(BINLOGCOMMAND_QPUSH_BACK)],
			binlogCmdMetrics[string(BINLOGCOMMAND_QPUSH_FRONT)],
			binlogCmdMetrics[string(BINLOGCOMMAND_QPOP_BACK)],
			binlogCmdMetrics[string(BINLOGCOMMAND_QPOP_FRONT)],
			binlogCmdMetrics[string(BINLOGCOMMAND_QSET)],
			binlogCmdMetrics[string(BINLOGCOMMAND_BEGIN)],
			binlogCmdMetrics[string(BINLOGCOMMAND_END)])
	}

	switch binlog.datatype {
	case BINLOGTYPE_NOOP:
		s.handleNoopRecv(binlog)
	case BINLOGTYPE_COPY:
		s.handleCopyRecv(binlog)
	case BINLOGTYPE_SYNC:
		s.handleSyncRecv(binlog)
	case BINLOGTYPE_CTRL:
		s.handleCtrlRecv(binlog)
	case BINLOGTYPE_MIRROR:
		s.handleMirrorRecv(binlog)
	default:
		log.Error("count is %d %d", s.status, count)
	}
	return
}


func (s *SSDBSalve) handleNoopRecv(binlog *Binlog) (err error) {
	//log.Info("handleNoopRecv:(%v)(%v)", binlog, string(binlog.body[1]))
	return nil
}

func (s *SSDBSalve) handleCopyRecv(binlog *Binlog) (err error) {
	//log.Info("handleCopyRecv:(%v)(%v)", binlog, string(binlog.body[1]))
	if binlog.cmdtype == BINLOGCOMMAND_BEGIN{
		log.Info("start handleCopyRecv:(%v)(%v)", binlog, string(binlog.body[1]))
		s.status = SALVESTATUS_COPY
		return
	}
	if binlog.cmdtype == BINLOGCOMMAND_END{
		log.Info("start handleCopyRecv:(%v)(%v)", binlog, string(binlog.body[1]))
		s.status = SALVESTATUS_SYNC
		return
	}
	s.handleRecvCmd(binlog)
	return nil
}

func (s *SSDBSalve) handleSyncRecv(binlog *Binlog) (err error) {
	//log.Info("handleSyncRecv:(%v)(%v)", binlog, string(binlog.body[1]))

	if s.status == SALVESTATUS_COPY{
		log.Info("start handleSyncRecv:(%v)(%v)", binlog, string(binlog.body[1]))
		s.status = SALVESTATUS_SYNC
	}
	s.handleRecvCmd(binlog)
	return nil
}

func (s *SSDBSalve) handleCtrlRecv(binlog *Binlog) (err error) {
	log.Info("handleCtrlRecv:(%v)(%v)", binlog, string(binlog.body[1]))
	return nil
}

func (s *SSDBSalve) handleMirrorRecv(binlog *Binlog) (err error) {
	log.Info("handleMirrorRecv:(%v)(%v)", binlog, string(binlog.body[1]))
	return nil
}

func (s *SSDBSalve) handleRecvCmd(binlog *Binlog) (err error) {
	//log.Info("handleRecvCmd: (%v)(%v)", binlog.cmd, binlog)
	*s.cmdsQueue <- binlog.cmd
	return nil
}

func (s *SSDBSalve) loadStatus() (err error) {
	return nil
}

func (s *SSDBSalve) saveStatus() (err error) {
	return nil
}

func (s *SSDBSalve) connectToMaster() (err error) {
	if s.connectRetry % 50 == 1 {
		log.Info("[%s][%d] connecting to master at %s...", s.id, s.connectRetry, s.from)
	}

	if s.c, err = Connect(s.from); err != nil{
		log.Error("[%s] failed to connect to master:%s (%v)", s.id, s.from, err)
		return
	}
	s.status = SALVESTATUS_INIT
	s.connectRetry = 0

	if s.auth != "" {
		//开始同步命令
		if err = s.c.Send("auth", s.auth); err != nil{
			log.Error("Auth error(%v)", err)
			return
		}
	}

	//开始同步命令
	if err = s.c.Send("sync140","0","","sync"); err != nil{
		log.Error("Send sync error(%v)", err)
		return
	}
	log.Info("[%s] ready to receive binlogs", s.id);

	return
}
