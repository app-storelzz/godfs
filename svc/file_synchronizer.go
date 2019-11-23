package svc

import (
	"container/list"
	"encoding/json"
	"errors"
	"github.com/boltdb/bolt"
	"github.com/hetianyi/godfs/api"
	"github.com/hetianyi/godfs/common"
	"github.com/hetianyi/godfs/util"
	"github.com/hetianyi/gox"
	"github.com/hetianyi/gox/file"
	"github.com/hetianyi/gox/logger"
	"github.com/hetianyi/gox/timer"
	"github.com/hetianyi/gox/uuid"
	"io"
	"strings"
	"time"
)

var (
	downloadBinlogPosKey = []byte("downloadBinlogPos")
	downloadBinlogPos    = 0
	fetchSize            = 50
)

func init() {

}

// InitFileSynchronization starts a timer job for file synchronization.
func InitFileSynchronization() {
	timer.Start(time.Second*5, time.Second*5, 0, func(t *timer.Timer) {
		config := common.GetConfigMap()
		for true {
			// filter group members.
			ins := filterGroupMembers(api.FilterInstances(common.ROLE_STORAGE), common.InitializedStorageConfiguration.Group)
			if ins.Len() == 0 {
				logger.Debug("no group member available")
				break
			}

			// get current binlog read position
			bs, err := config.GetConfig(string(downloadBinlogPosKey))
			if err != nil {
				logger.Debug(err)
				break
			}

			if n, _ := fromBinlogPos(bs, false); n == 0 {
				break
			}
		}
	})
	retryFiles()
}

func fromBinlogPos(bs []byte, isRetry bool) (int, []common.BingLogDTO) {

	ret := &common.BinlogQueryDTO{}
	if bs != nil && len(bs) > 0 {
		if err := json.Unmarshal(bs, ret); err != nil {
			logger.Debug(err)
			return 0, nil
		}
	}

	old := *ret

	bls, nOffset, err := writableBinlogManager.Read(ret.FileIndex, ret.Offset, 50)
	if err != nil {
		logger.Debug(err)
		return len(bls), bls
	}
	ret.Offset = nOffset

	if writableBinlogManager.GetCurrentIndex() > ret.FileIndex && len(bls) == 0 {
		ret.FileIndex = ret.FileIndex + 1
		ret.Offset = 0
	}

	// save config
	// download files
	failed := syncFiles(bls)

	if !isRetry {
		// save binlog position and fail position.
		if err := saveDownloadStateConfig(ret, &old, failed); err != nil {
			logger.Debug(err)
		}
	}

	return len(bls), bls
}

// retryFiles starts a timer job which retries to synchronize files failed before.
//
//
func retryFiles() {

	temp := list.New()
	offset := 0
	donePos := list.New()

	var clearFun = func() {
		// clear
		for temp.Front() != nil {
			temp.Remove(temp.Front())
		}
	}

	timer.Start(time.Second*5, time.Second*5, 0, func(t *timer.Timer) {

		offset = 0

		for true {
			clearFun()
			// load 100 binlog position once a time.
			if err := common.GetConfigMap().IteratorFailedBinlog(func(c *bolt.Cursor) error {
				skipped := 0
				for k, _ := c.First(); k != nil; k, _ = c.Next() {
					skipped++
					if skipped < offset {
						continue
					}
					temp.PushBack(k)
				}
				offset += temp.Len()
				return nil
			}); err != nil {
				logger.Debug(err)
				break
			}

			if temp.Len() == 0 {
				break
			}

			// retry download files.
			gox.WalkList(temp, func(item interface{}) bool {
				k := item.([]byte)
				_, bls := fromBinlogPos(k, true)
				// check if all binlog of this position are finished.
				finished := 0
				for _, v := range bls {
					c, err := Contains(v.FileId)
					if err != nil {
						logger.Debug(err)
						break
					}
					if c {
						finished++
					}
				}
				// all file were synchronized of this binlog position.
				if finished == len(bls) {
					donePos.PushBack(item)
				}
				return false
			})

			if donePos.Len() > 0 {
				removeSuccess := 0
				// remove finished positions.
				common.GetConfigMap().BatchUpdate(func(tx *bolt.Tx) error {
					gox.WalkList(donePos, func(item interface{}) bool {
						if err := tx.Bucket([]byte(common.BUCKET_KEY_FAILED_BINLOG_POS)).Delete(item.([]byte)); err != nil {
							logger.Debug("error remove failed key: ", err)
							return false
						}
						removeSuccess++
						return false
					})
					return nil
				})
				offset -= removeSuccess
			}
		}
	})
}

// syncFiles synchronizes files by binlogs.
func syncFiles(bls []common.BingLogDTO) int {
	if len(bls) == 0 {
		return 0
	}

	logger.Debug("load ", len(bls), " binlogs")

	failed := 0
	for _, v := range bls {
		if err := syncFile(&v, nil); err != nil {
			failed++
		}
	}

	return failed
}

// syncFile synchronizes a single file.
func syncFile(binlog *common.BingLogDTO, server *common.Server) error {

	if binlog == nil {
		return nil
	}

	fInfo, _, err := util.ParseAlias(binlog.FileId, common.InitializedStorageConfiguration.Secret)
	if err != nil {
		return errors.New("cannot parse alias: " + binlog.FileId)
	}

	if server == nil {
		ins := api.FilterInstances(common.ROLE_STORAGE)
		if ins.Len() == 0 {
			return errors.New("no storage server available")
		}

		// filter group members.
		ins = filterGroupMembers(ins, common.InitializedStorageConfiguration.Group)

		// download from source server first.
		var srcServer common.Server
		gox.WalkList(ins, func(item interface{}) bool {
			if item.(*common.Instance).InstanceId == binlog.SourceInstance {
				srcServer = item.(*common.Instance).Server
				return true
			}
			return false
		})

		var lasErr error

		if err := syncFile(binlog, &srcServer); err != nil {
			lasErr = err
		}

		if lasErr == nil {
			return nil
		}

		logger.Debug("cannot download from source server, try other servers.")

		// fallback, download from other servers.
		for ele := ins.Front(); ele != nil; ele = ele.Next() {
			s := ele.Value.(*common.Instance)
			if s.InstanceId == binlog.SourceInstance {
				continue
			}

			server = &s.Server

			logger.Debug("trying to download from ",
				server.ConnectionString(), "(", server.InstanceId, ")")

			if err := syncFile(binlog, &s.Server); err != nil {
				lasErr = err
				continue
			}
			// upload success, clear error.
			lasErr = nil
			break
		}
		return lasErr
	}

	logger.Debug("begin to synchronize file ", binlog.FileId, " from ",
		server.ConnectionString(), "(", server.InstanceId, ")")

	return clientAPI.DownloadFrom(binlog.FileId, 0, -1, server, func(body io.Reader, bodyLength int64) error {
		tmpFileName := common.InitializedStorageConfiguration.TmpDir + "/" + uuid.UUID()
		out, err := file.CreateFile(tmpFileName)
		if err != nil {
			return err
		}
		defer func() {
			out.Close()
			file.Delete(tmpFileName)
		}()

		proxy := &DigestProxyWriter{
			crcH: util.CreateCrc32Hash(),
			md5H: util.CreateMd5Hash(),
			out:  out,
		}

		logger.Debug("copy file")
		_, err = io.Copy(proxy, io.LimitReader(body, bodyLength))
		if err != nil {
			return err
		}

		logger.Debug("write tail")
		// write reference count mark.
		_, err = out.Write(tailRefCount)
		if err != nil {
			return err
		}
		out.Close()

		targetLoc := common.InitializedStorageConfiguration.DataDir + "/" + fInfo.Path[0:strings.LastIndex(fInfo.Path, "/")]
		targetFile := common.InitializedStorageConfiguration.DataDir + "/" + fInfo.Path

		if !file.Exists(targetLoc) {
			if err := file.CreateDirs(targetLoc); err != nil {
				return err
			}
		}

		if !file.Exists(targetFile) {
			logger.Debug("file not exists, move to target dir.")
			if err := file.MoveFile(tmpFileName, targetFile); err != nil {
				return err
			}
		} else {
			logger.Debug("file already exists, increasing reference count.")
			// increase file reference count.
			if err = updateFileReferenceCount(targetFile, 1); err != nil {
				return err
			}
		}

		logger.Debug("add dataset...")
		if err := Add(binlog.FileId); err != nil {
			return errors.New("error writing dataset: " + err.Error())
		}
		logger.Debug("add dataset success")
		logger.Debug("download success")
		return nil
	})
}

func filterGroupMembers(members *list.List, group string) *list.List {
	ret := list.New()
	gox.WalkList(members, func(item interface{}) bool {
		if item.(*common.Instance).Attributes["group"] == group &&
			item.(*common.Instance).InstanceId != common.InitializedStorageConfiguration.InstanceId {
			ret.PushBack(item.(*common.Instance))
		}
		return false
	})
	return ret
}

func saveDownloadStateConfig(n *common.BinlogQueryDTO, o *common.BinlogQueryDTO, failed int) error {
	bs1, err := json.Marshal(n)
	if err != nil {
		return nil
	}
	bs2, err := json.Marshal(o)
	if err != nil {
		return nil
	}
	config := common.GetConfigMap()

	return config.BatchUpdate(func(tx *bolt.Tx) error {
		b1 := tx.Bucket([]byte(common.BUCKET_KEY_CONFIGMAP))
		err := b1.Put(downloadBinlogPosKey, bs1)
		if err != nil {
			return err
		}
		// mark failed binlog position
		if failed > 0 {
			b2 := tx.Bucket([]byte(common.BUCKET_KEY_FAILED_BINLOG_POS))
			if b2.Get(bs2) == nil {
				err = b2.Put(bs2, []byte{1})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}