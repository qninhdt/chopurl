package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type IdAllocator struct {
	segmentId     int   // requested segment id from etcd
	nextSegmentId int   // pre-requested segment id from etcd
	length        int   // length of the id queue
	frontQueue    []int // main queue of ids, ids are allocated from this queue
	backQueue     []int // backup queue of ids, ids are pre-requested from etcd
	lock          sync.Mutex
	etcdClient    *clientv3.Client
	options       *IdAllocatorOptions
	etcdOptions   *EtcdOptions
}

type IdAllocatorOptions struct {
	SegmentSize     int     `mapstructure:"segment_size"`      // size of the segment
	QueueThreshold  float32 `mapstructure:"queue_threshold"`   // threshold for pre-requesting a new segment
	SegmentCountKey string  `mapstructure:"segment_count_key"` // key for the segment count in etcd
	SegmentMapKey   string  `mapstructure:"segment_map_key"`   // key for the segment map in etcd
	MaxSegmentCount int     `mapstructure:"max_segment_count"` // maximum number of segments
}

type EtcdOptions struct {
	Address        string        `mapstructure:"address"`         // etcd address
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"` // timeout in seconds
	RequestTimeout time.Duration `mapstructure:"request_timeout"` // timeout in seconds
}

func NewIdAllocator(idAllocOptions *IdAllocatorOptions, etcdOptions *EtcdOptions) (*IdAllocator, func(), error) {
	// create a new etcd client
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdOptions.Address},
		DialTimeout: etcdOptions.ConnectTimeout,
	})

	if err != nil {
		return nil, nil, errors.New("failed to connect to etcd: " + err.Error())
	}

	log.Println("Connected to etcd at", etcdOptions.Address)

	// init ectd segment count if not exists
	ctx, cancel := context.WithTimeout(context.Background(), etcdOptions.ConnectTimeout)
	defer cancel()

	// Check if the segment count key already exists
	resp, err := etcdClient.Get(ctx, idAllocOptions.SegmentCountKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get segment count key: %v", err)
	}
	if len(resp.Kvs) == 0 {
		// If it doesn't exist, initialize it with the maximum segment count
		_, err = etcdClient.Put(ctx, idAllocOptions.SegmentCountKey, strconv.Itoa(idAllocOptions.MaxSegmentCount))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize segment count key: %v", err)
		}
		log.Println("Initialized segment count key with value:", idAllocOptions.MaxSegmentCount)
	}

	// Initialize the IdAllocator with the given options
	idAllocator := &IdAllocator{
		segmentId:     0,
		nextSegmentId: 0,
		length:        0,
		frontQueue:    make([]int, idAllocOptions.SegmentSize),
		backQueue:     make([]int, idAllocOptions.SegmentSize),
		lock:          sync.Mutex{},
		etcdClient:    etcdClient,
		options:       idAllocOptions,
		etcdOptions:   etcdOptions,
	}

	// initialize the back queue with the segment size
	err = idAllocator.requestSegment()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to allocate initial segment: %v", err)
	}

	// swap the front and back queues
	idAllocator.switchQueue()

	log.Println("Allocated initial segment with ID:", idAllocator.segmentId)

	return idAllocator, func() {
		if err := idAllocator.etcdClient.Close(); err != nil {
			log.Fatal("failed to close etcd client: " + err.Error())
		}
	}, nil
}

func (ia *IdAllocator) Pop() (int64, error) {
	// lock the queue to prevent concurrent access
	ia.lock.Lock()
	defer ia.lock.Unlock()

	// check if we need to pre-request a new segment
	threshold := int(ia.options.QueueThreshold * float32(ia.options.SegmentSize))
	if ia.length == threshold {
		// request a new segment in the background to avoid blocking
		job := func() {
			if err := ia.requestSegment(); err != nil {
				log.Fatal("failed to request new segment:", err)
			} else {
				log.Println("Requested new segment with ID:", ia.nextSegmentId)
			}
		}
		go job()
	}

	// check if we need to switch the queue
	if ia.length == 0 {
		if err := ia.switchQueue(); err != nil {
			return 0, err
		}

		log.Println("Switched to new segment with ID:", ia.segmentId)
	}

	// select random id from the queue
	idx := rand.Intn(ia.length)

	// remove the ID from the queue by swapping it with the last element
	localId := ia.frontQueue[idx]
	ia.frontQueue[idx] = ia.frontQueue[ia.length-1]
	ia.length--

	// convert to global 64-bit ID
	// minus 1 to convert from 1-based to 0-based index
	globalId := int64(ia.segmentId-1)*int64(ia.options.SegmentSize) + int64(localId-1) + 1

	return globalId, nil
}

func (ia *IdAllocator) switchQueue() error {
	if ia.length > 0 {
		return errors.New("cannot switch queues while there are IDs available")
	}

	if ia.nextSegmentId == -1 {
		return errors.New("no next segment ID available")
	}

	// switch the front and back queues
	ia.frontQueue, ia.backQueue = ia.backQueue, ia.frontQueue
	ia.length = ia.options.SegmentSize
	ia.segmentId = ia.nextSegmentId
	ia.nextSegmentId = 0

	// clear the back queue
	for i := 0; i < ia.options.SegmentSize; i++ {
		ia.backQueue[i] = 0
	}

	return nil
}

// `requestSegment` requests a random segment ID from etcd using Fisher-Yates
// shuffle algorithm. It atomically updates the segment count and remap
// the selected index to the last position in the segment.
func (ia *IdAllocator) requestSegment() error {
	if ia.nextSegmentId != 0 {
		return errors.New("next segment ID already allocated")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ia.etcdOptions.RequestTimeout)
	defer cancel()

	// Get the current remaining count
	resp, err := ia.etcdClient.Get(ctx, ia.options.SegmentCountKey)

	if err != nil {
		return fmt.Errorf("failed to get remaining count: %v", err)
	}
	if len(resp.Kvs) == 0 {
		return fmt.Errorf("remaining count not found, generator not initialized")
	}

	segmentCount, err := strconv.Atoi(string(resp.Kvs[0].Value))
	if err != nil {
		return fmt.Errorf("invalid remaining count: %v", err)
	}

	if segmentCount <= 0 {
		return fmt.Errorf("all numbers have been generated")
	}

	// Choose a random index from the remaining set
	randomIndex := rand.Intn(segmentCount) + 1

	// Check if this position has been remapped
	remapKey := fmt.Sprintf("%s/%d", ia.options.SegmentMapKey, randomIndex)
	remapResp, err := ia.etcdClient.Get(ctx, remapKey)
	if err != nil {
		return fmt.Errorf("failed to check remap: %v", err)
	}

	// Determine our result value
	var result int
	if len(remapResp.Kvs) > 0 {
		result, err = strconv.Atoi(string(remapResp.Kvs[0].Value))
		if err != nil {
			return fmt.Errorf("invalid remap value: %v", err)
		}
	} else {
		result = randomIndex
	}

	// Get the value for the last position (if it exists)
	lastPosKey := fmt.Sprintf("%s/%d", ia.options.SegmentMapKey, segmentCount)
	lastPosResp, err := ia.etcdClient.Get(ctx, lastPosKey)
	if err != nil {
		return fmt.Errorf("failed to get last position: %v", err)
	}

	// Build the transaction for atomically updating all values
	txn := ia.etcdClient.Txn(ctx)

	// Make sure remaining count hasn't changed (optimistic concurrency control)
	txn = txn.If(clientv3.Compare(clientv3.Value(ia.options.SegmentCountKey), "=", string(resp.Kvs[0].Value)))

	// Operations to perform if the check succeeds
	var thenOps []clientv3.Op

	// Update remaining count
	thenOps = append(thenOps, clientv3.OpPut(ia.options.SegmentCountKey, strconv.Itoa(segmentCount-1)))

	// Update the remap for the randomly selected index
	if len(lastPosResp.Kvs) > 0 {
		// The last position was remapped, use that value
		thenOps = append(thenOps, clientv3.OpPut(remapKey, string(lastPosResp.Kvs[0].Value)))
	} else {
		// Use the last position itself
		thenOps = append(thenOps, clientv3.OpPut(remapKey, strconv.Itoa(segmentCount)))
	}

	// Execute the transaction
	txnResp, err := txn.Then(thenOps...).Else(clientv3.OpGet(ia.options.SegmentCountKey)).Commit()
	if err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	if !txnResp.Succeeded {
		// Transaction failed because remaining count changed, retry
		return ia.requestSegment()
	}

	// print the remaining count
	log.Println("Remaining segment count:", segmentCount-1)

	// Update the segment ID and fill the back queue
	ia.nextSegmentId = result

	// fill the back queue with IDs from the selected segment
	// the IDs are 1-based, so we start from 1
	for i := 0; i < ia.options.SegmentSize; i++ {
		ia.backQueue[i] = i + 1
	}

	return nil
}
