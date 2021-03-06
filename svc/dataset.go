package svc

import (
	"github.com/hetianyi/godfs/common"
	"github.com/hetianyi/gox/logger"
	"github.com/hetianyi/gox/set"
	"sync"
)

var (
	dataset   *set.DataSet
	initLock  *sync.Mutex
	writeLock *sync.Mutex
	initd     = false
)

func init() {
	initLock = new(sync.Mutex)
	writeLock = new(sync.Mutex)
}

// initDataSet initializes database which stores fileId.
func initDataSet() error {
	initLock.Lock()
	defer initLock.Unlock()

	if initd {
		logger.Warn("dataset is already initialized")
		return nil
	}
	initd = true

	if common.BootAs != common.BOOT_TRACKER && common.BootAs != common.BOOT_STORAGE {
		logger.Fatal("cannot init dataset: invalid boot role ", common.BootAs)
	}

	// slotSize is the slot size of the set,
	// it loads the size of slots bytes in memory, so be careful.
	//
	// storage: 16777216
	// tracker: 33554432
	slotNum := 1 << (23 + common.BootAs)
	//a := 1 << 23

	logger.Debug("slot number: ", slotNum)

	// slotSize is size of fileId in bytes.
	slotSize := common.FILE_ID_SIZE

	logger.Debug("slot size: ", slotSize)

	dataDir := ""
	if common.BootAs == common.BOOT_TRACKER {
		dataDir = common.InitializedTrackerConfiguration.DataDir
	} else {
		dataDir = common.InitializedStorageConfiguration.DataDir
	}

	m, err := set.NewFileMap(slotNum, 8, dataDir+"/index")
	if err != nil {
		return err
	}
	a, err := set.NewAppendFile(slotSize, 2, dataDir+"/aof")
	if err != nil {
		return err
	}

	dataset = set.NewDataSet(m, a)

	logger.Debug("dataset initializes success")

	return nil
}

// Add adds fileId to dataset database.
func Add(fileId string) error {
	return dataset.Add([]byte(fileId))
}

// Add removes fileId from dataset database.
func Remove(fileId string) (bool, error) {
	return dataset.Remove([]byte(fileId))
}

// Contains checks if the fileId exists in dataset database.
func Contains(fileId string) (bool, error) {
	return dataset.Contains([]byte(fileId))
}

// DoIfNotExist does work if the fileId not exists.
func DoIfNotExist(fileId string, work func() error) error {
	writeLock.Lock()
	defer writeLock.Unlock()

	c, err := dataset.Contains([]byte(fileId))
	if err != nil {
		return err
	}
	if !c {
		return work()
	}
	return nil
}
