package snowflake

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

type ISnowflake interface {
	NextId() int64
}

// SnowWorkerM1 .
type SnowWorkerM1 struct {
	BaseTime          int64  // 基础时间
	WorkerId          uint16 // 机器码
	WorkerIdBitLength byte   // 机器码位长
	SeqBitLength      byte   // 自增序列数位长
	MaxSeqNumber      uint32 // 最大序列数（含）
	MinSeqNumber      uint32 // 最小序列数（含）
	TopOverCostCount  uint32 // 最大漂移次数
	TimestampShift    byte
	CurrentSeqNumber  uint32

	LastTimeTick           int64
	TurnBackTimeTick       int64
	TurnBackIndex          byte
	IsOverCost             bool
	OverCostCountInOneTerm uint32
	// GenCountInOneTerm      uint32
	// TermIndex              uint32

	sync.Mutex
}

type SnowWorkerM2 struct {
	*SnowWorkerM1
}

// NewSnowWorkerM1 .
func NewSnowWorkerM1(options *SnowflakeOptions) ISnowflake {
	var workerIdBitLength byte
	var seqBitLength byte
	var maxSeqNumber uint32

	// 1.BaseTime
	var baseTime int64
	if options.BaseTime != 0 {
		baseTime = options.BaseTime
	} else {
		baseTime = 1582136402000
	}

	// 2.WorkerIdBitLength
	if options.WorkerIdBitLength == 0 {
		workerIdBitLength = 6
	} else {
		workerIdBitLength = options.WorkerIdBitLength
	}

	// 3.WorkerId
	var workerId = options.WorkerId

	// 4.SeqBitLength
	if options.SeqBitLength == 0 {
		seqBitLength = 6
	} else {
		seqBitLength = options.SeqBitLength
	}

	// 5.MaxSeqNumber
	if options.MaxSeqNumber <= 0 {
		maxSeqNumber = (1 << seqBitLength) - 1
	} else {
		maxSeqNumber = options.MaxSeqNumber
	}

	// 6.MinSeqNumber
	var minSeqNumber = options.MinSeqNumber

	// 7.TopOverCostCount
	var topOverCostCount = options.TopOverCostCount
	// if topOverCostCount == 0 {
	// 	topOverCostCount = 2000
	// }

	// 8.Others
	timestampShift := (byte)(workerIdBitLength + seqBitLength)
	currentSeqNumber := minSeqNumber

	return &SnowWorkerM1{
		BaseTime:          baseTime,
		WorkerIdBitLength: workerIdBitLength,
		WorkerId:          workerId,
		SeqBitLength:      seqBitLength,
		MaxSeqNumber:      maxSeqNumber,
		MinSeqNumber:      minSeqNumber,
		TopOverCostCount:  topOverCostCount,
		TimestampShift:    timestampShift,
		CurrentSeqNumber:  currentSeqNumber,

		LastTimeTick:           0,
		TurnBackTimeTick:       0,
		TurnBackIndex:          0,
		IsOverCost:             false,
		OverCostCountInOneTerm: 0,
		// _GenCountInOneTerm:      0,
		// _TermIndex:              0,
	}
}

// DoGenIDAction .
func (m1 *SnowWorkerM1) DoGenIdAction(arg *OverCostAction) {

}

func (m1 *SnowWorkerM1) BeginOverCostAction(useTimeTick int64) {

}

func (m1 *SnowWorkerM1) EndOverCostAction(useTimeTick int64) {
	// if m1._TermIndex > 10000 {
	// 	m1._TermIndex = 0
	// }
}

func (m1 *SnowWorkerM1) BeginTurnBackAction(useTimeTick int64) {

}

func (m1 *SnowWorkerM1) EndTurnBackAction(useTimeTick int64) {

}

func (m1 *SnowWorkerM1) NextOverCostId() int64 {
	currentTimeTick := m1.GetCurrentTimeTick()
	if currentTimeTick > m1.LastTimeTick {
		// m1.EndOverCostAction(currentTimeTick)
		m1.LastTimeTick = currentTimeTick
		m1.CurrentSeqNumber = m1.MinSeqNumber
		m1.IsOverCost = false
		m1.OverCostCountInOneTerm = 0
		// m1._GenCountInOneTerm = 0
		return m1.CalcId(m1.LastTimeTick)
	}
	if m1.OverCostCountInOneTerm >= m1.TopOverCostCount {
		// m1.EndOverCostAction(currentTimeTick)
		m1.LastTimeTick = m1.GetNextTimeTick()
		m1.CurrentSeqNumber = m1.MinSeqNumber
		m1.IsOverCost = false
		m1.OverCostCountInOneTerm = 0
		// m1._GenCountInOneTerm = 0
		return m1.CalcId(m1.LastTimeTick)
	}
	if m1.CurrentSeqNumber > m1.MaxSeqNumber {
		m1.LastTimeTick++
		m1.CurrentSeqNumber = m1.MinSeqNumber
		m1.IsOverCost = true
		m1.OverCostCountInOneTerm++
		// m1._GenCountInOneTerm++

		return m1.CalcId(m1.LastTimeTick)
	}

	// m1._GenCountInOneTerm++
	return m1.CalcId(m1.LastTimeTick)
}

// NextNormalID .
func (m1 *SnowWorkerM1) NextNormalId() int64 {
	currentTimeTick := m1.GetCurrentTimeTick()
	if currentTimeTick < m1.LastTimeTick {
		if m1.TurnBackTimeTick < 1 {
			m1.TurnBackTimeTick = m1.LastTimeTick - 1
			m1.TurnBackIndex++
			// 每毫秒序列数的前5位是预留位，0用于手工新值，1-4是时间回拨次序
			// 支持4次回拨次序（避免回拨重叠导致ID重复），可无限次回拨（次序循环使用）。
			if m1.TurnBackIndex > 4 {
				m1.TurnBackIndex = 1
			}
			m1.BeginTurnBackAction(m1.TurnBackTimeTick)
		}

		// time.Sleep(time.Duration(1) * time.Millisecond)
		return m1.CalcTurnBackId(m1.TurnBackTimeTick)
	}

	// 时间追平时，_TurnBackTimeTick清零
	if m1.TurnBackTimeTick > 0 {
		m1.EndTurnBackAction(m1.TurnBackTimeTick)
		m1.TurnBackTimeTick = 0
	}

	if currentTimeTick > m1.LastTimeTick {
		m1.LastTimeTick = currentTimeTick
		m1.CurrentSeqNumber = m1.MinSeqNumber
		return m1.CalcId(m1.LastTimeTick)
	}

	if m1.CurrentSeqNumber > m1.MaxSeqNumber {
		m1.BeginOverCostAction(currentTimeTick)
		// m1._TermIndex++
		m1.LastTimeTick++
		m1.CurrentSeqNumber = m1.MinSeqNumber
		m1.IsOverCost = true
		m1.OverCostCountInOneTerm = 1
		// m1._GenCountInOneTerm = 1

		return m1.CalcId(m1.LastTimeTick)
	}

	return m1.CalcId(m1.LastTimeTick)
}

// CalcID .
func (m1 *SnowWorkerM1) CalcId(useTimeTick int64) int64 {
	result := int64(useTimeTick<<m1.TimestampShift) + int64(m1.WorkerId<<m1.SeqBitLength) + int64(m1.CurrentSeqNumber)
	m1.CurrentSeqNumber++
	return result
}

// CalcTurnBackID .
func (m1 *SnowWorkerM1) CalcTurnBackId(useTimeTick int64) int64 {
	result := int64(useTimeTick<<m1.TimestampShift) + int64(m1.WorkerId<<m1.SeqBitLength) + int64(m1.TurnBackIndex)
	m1.TurnBackTimeTick--
	return result
}

// GetCurrentTimeTick .
func (m1 *SnowWorkerM1) GetCurrentTimeTick() int64 {
	var millis = time.Now().UnixNano() / 1e6
	return millis - m1.BaseTime
}

// GetNextTimeTick .
func (m1 *SnowWorkerM1) GetNextTimeTick() int64 {
	tempTimeTicker := m1.GetCurrentTimeTick()
	for tempTimeTicker <= m1.LastTimeTick {
		time.Sleep(time.Duration(1) * time.Millisecond)
		tempTimeTicker = m1.GetCurrentTimeTick()
	}
	return tempTimeTicker
}

// GenSnowflakeId .
func (m1 *SnowWorkerM1) NextId() int64 {
	m1.Lock()
	defer m1.Unlock()
	if m1.IsOverCost {
		return m1.NextOverCostId()
	} else {
		return m1.NextNormalId()
	}
}

func NewSnowWorkerM2(options *SnowflakeOptions) ISnowflake {
	return &SnowWorkerM2{
		NewSnowWorkerM1(options).(*SnowWorkerM1),
	}
}

func (m2 SnowWorkerM2) NextId() int64 {
	m2.Lock()
	defer m2.Unlock()
	currentTimeTick := m2.GetCurrentTimeTick()
	if m2.LastTimeTick == currentTimeTick {
		m2.CurrentSeqNumber++
		if m2.CurrentSeqNumber > m2.MaxSeqNumber {
			m2.CurrentSeqNumber = m2.MinSeqNumber
			currentTimeTick = m2.GetNextTimeTick()
		}
	} else {
		m2.CurrentSeqNumber = m2.MinSeqNumber
	}
	if currentTimeTick < m2.LastTimeTick {
		fmt.Println("Time error for {0} milliseconds", strconv.FormatInt(m2.LastTimeTick-currentTimeTick, 10))
	}
	m2.LastTimeTick = currentTimeTick
	result := int64(currentTimeTick<<m2.TimestampShift) + int64(m2.WorkerId<<m2.SeqBitLength) + int64(m2.CurrentSeqNumber)
	return result
}
