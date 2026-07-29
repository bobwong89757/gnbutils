package sharding

import (
	"sync"
	"testing"
)

func TestSnowflakeNextIDUnique(t *testing.T) {
	sf, err := NewSnowflake(1, 1, 2000)
	if err != nil {
		t.Fatal(err)
	}

	seen := make(map[int64]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id, err := sf.NextID()
		if err != nil {
			t.Fatalf("NextID: %v", err)
		}
		if id <= 0 {
			t.Fatalf("id must be positive, got %d", id)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id %d", id)
		}
		seen[id] = struct{}{}
	}
}

func TestSnowflakeConcurrentUnique(t *testing.T) {
	sf, err := NewSnowflake(1, 1, 2000)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	const perWorker = 200
	ids := make(chan int64, workers*perWorker)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				id, err := sf.NextID()
				if err != nil {
					t.Errorf("NextID: %v", err)
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int64]struct{})
	for id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id %d", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != workers*perWorker {
		t.Fatalf("want %d ids, got %d", workers*perWorker, len(seen))
	}
}

func TestInitIDGeneratorSnowflake(t *testing.T) {
	setGlobalIDGenerator(nil)
	cfg := &ShardingConfig{
		PrimaryKeyGenerator: "snowflake",
		Snowflake: SnowflakeConfig{
			WorkerID:     1,
			DatacenterID: 1,
		},
	}
	if err := initIDGenerator(cfg); err != nil {
		t.Fatal(err)
	}
	id, err := NextUint64()
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}
}
