package lib_tracker

import (
    "util/logger"
    "strconv"
    "net"
    "util/pool"
    "util/common"
    "lib_common"
    "time"
    "lib_common/bridge"
    "io"
)

var p, _ = pool.NewPool(1000, 100000)


var secret string

// Start service and listen
// 1. Start task for upload listen
// 2. Start task for communication with tracker
func StartService(config map[string] string) {
    port := config["port"]
    secret = config["secret"]
    go ExpirationDetection()
    startTrackerService(port)
}

// Tracker server start listen
func startTrackerService(port string) {
    pt := lib_common.ParsePort(port)
    if pt > 0 {
        tryTimes := 0
        for {
            common.Try(func() {
                listener, e := net.Listen("tcp", ":" + strconv.Itoa(pt))
                logger.Info("service listening on port:", pt)
                if e != nil {
                    panic(e)
                } else {
                    // keep accept connections.
                    for {
                        conn, e1 := listener.Accept()
                        if e1 == nil {
                            ee := p.Exec(func() {
                                clientHandler(conn)
                            })
                            // maybe the poll is full
                            if ee != nil {
                                bridge.Close(conn)
                            }
                        } else {
                            logger.Info("accept new conn error", e1)
                            if conn != nil {
                                bridge.Close(conn)
                            }
                        }
                    }
                }
            }, func(i interface{}) {
                logger.Error("["+ strconv.Itoa(tryTimes) +"] error shutdown service duo to:", i)
                time.Sleep(time.Second * 10)
            })
        }
    }
}



// accept a new connection for file upload
// the connection will keep till it is broken
func clientHandler(conn net.Conn) {
    // anyway defer close conn
    defer func() {
        logger.Debug("close connection from server")
        bridge.Close(conn)
    }()
    var storageClient *bridge.OperationRegisterStorageClientRequest
    common.Try(func() {
        connBridge := bridge.NewBridge(conn)
        for {
            error := connBridge.ReceiveRequest(func(request *bridge.Meta, in io.ReadCloser) error {
                //return requestRouter(request, &bodyBuff, md, connBridge, conn)
                if request.Err != nil {
                    return request.Err
                }
                // route
                if request.Operation == bridge.O_CONNECT {
                    return validateClientHandler(request, connBridge)
                } else if request.Operation == bridge.O_REG_STORAGE {
                    var e error
                    storageClient, e = registerStorageClientHandler(request, conn, connBridge)
                    return e
                } else {
                    return bridge.OPERATION_NOT_SUPPORT_ERROR
                }
                return nil
            })
            if error != nil {
                logger.Error(error)
                break
            }
        }
    }, func(i interface{}) {
        logger.Error("connection error:", i)
    })
    FutureExpireStorageServer(storageClient)
}

