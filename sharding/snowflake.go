package sharding

import (
	"fmt"
	"sync"
	"time"
)

// 与 MyBatis-Plus DefaultIdentifierGenerator / Twitter Snowflake 布局一致。
const (
	defaultSnowflakeEpoch                       = int64(1288834974657) // 2010-11-04 09:42:54 UTC
	defaultSnowflakeWorkerID                    = int64(1)
	defaultSnowflakeDatacenterID                = int64(1)
	defaultSnowflakeMaxTolerateTimeDifferenceMs = int64(2000)

	snowflakeWorkerIDBits     = int64(5)
	snowflakeDatacenterIDBits = int64(5)
	snowflakeSequenceBits     = int64(12)

	snowflakeMaxWorkerID     = int64(-1) ^ (int64(-1) << snowflakeWorkerIDBits)
	snowflakeMaxDatacenterID = int64(-1) ^ (int64(-1) << snowflakeDatacenterIDBits)
	snowflakeSequenceMask    = int64(-1) ^ (int64(-1) << snowflakeSequenceBits)

	snowflakeWorkerIDShift      = snowflakeSequenceBits
	snowflakeDatacenterIDShift  = snowflakeSequenceBits + snowflakeWorkerIDBits
	snowflakeTimestampLeftShift = snowflakeSequenceBits + snowflakeWorkerIDBits + snowflakeDatacenterIDBits
)

// SnowflakeConfig 雪花 ID 配置（对齐 game-server / MyBatis-Plus global-config.sequence）
type SnowflakeConfig struct {
	WorkerID                    int64 `yaml:"worker_id"`
	DatacenterID                int64 `yaml:"datacenter_id"`
	MaxTolerateTimeDifferenceMs int64 `yaml:"max_tolerate_time_difference_ms"`
}

// Snowflake 线程安全雪花 ID 生成器。
type Snowflake struct {
	mu                          sync.Mutex
	epoch                       int64
	workerID                    int64
	datacenterID                int64
	maxTolerateTimeDifferenceMs int64
	sequence                    int64
	lastTimestamp               int64
}

func NewSnowflake(workerID, datacenterID, maxTolerateTimeDifferenceMs int64) (*Snowflake, error) {
	if workerID < 0 || workerID > snowflakeMaxWorkerID {
		return nil, fmt.Errorf("snowflake worker_id out of range [0, %d]", snowflakeMaxWorkerID)
	}
	if datacenterID < 0 || datacenterID > snowflakeMaxDatacenterID {
		return nil, fmt.Errorf("snowflake datacenter_id out of range [0, %d]", snowflakeMaxDatacenterID)
	}
	if maxTolerateTimeDifferenceMs <= 0 {
		maxTolerateTimeDifferenceMs = defaultSnowflakeMaxTolerateTimeDifferenceMs
	}
	return &Snowflake{
		epoch:                       defaultSnowflakeEpoch,
		workerID:                    workerID,
		datacenterID:                datacenterID,
		maxTolerateTimeDifferenceMs: maxTolerateTimeDifferenceMs,
		lastTimestamp:               -1,
	}, nil
}

func NewSnowflakeFromConfig(cfg SnowflakeConfig) (*Snowflake, error) {
	workerID := cfg.WorkerID
	if workerID == 0 {
		workerID = defaultSnowflakeWorkerID
	}
	datacenterID := cfg.DatacenterID
	if datacenterID == 0 {
		datacenterID = defaultSnowflakeDatacenterID
	}
	return NewSnowflake(workerID, datacenterID, cfg.MaxTolerateTimeDifferenceMs)
}

func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := s.currentMillis()
	if timestamp < s.lastTimestamp {
		offset := s.lastTimestamp - timestamp
		if offset <= s.maxTolerateTimeDifferenceMs {
			timestamp = s.lastTimestamp
		} else {
			return 0, fmt.Errorf("snowflake clock moved backwards, refuse for %dms", offset)
		}
	}

	if timestamp == s.lastTimestamp {
		s.sequence = (s.sequence + 1) & snowflakeSequenceMask
		if s.sequence == 0 {
			timestamp = s.waitNextMillis(s.lastTimestamp)
		}
	} else {
		s.sequence = 0
	}

	s.lastTimestamp = timestamp

	id := ((timestamp - s.epoch) << snowflakeTimestampLeftShift) |
		(s.datacenterID << snowflakeDatacenterIDShift) |
		(s.workerID << snowflakeWorkerIDShift) |
		s.sequence
	return id, nil
}

func (s *Snowflake) NextUint64() (uint64, error) {
	id, err := s.NextID()
	if err != nil {
		return 0, err
	}
	if id < 0 {
		return 0, fmt.Errorf("snowflake generated negative id: %d", id)
	}
	return uint64(id), nil
}

func (s *Snowflake) currentMillis() int64 {
	return time.Now().UnixMilli()
}

func (s *Snowflake) waitNextMillis(lastTimestamp int64) int64 {
	timestamp := s.currentMillis()
	for timestamp <= lastTimestamp {
		timestamp = s.currentMillis()
	}
	return timestamp
}
