package common

import (
	"dt-server/common/logger"
	"time"

	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"

	"github.com/jmoiron/sqlx"
)

// 初始化master db
func InitDB(dsn string, maxIdleConn, maxOpenConn int) *sqlx.DB {

	db, err := sqlx.Connect("mysql", dsn+"&parseTime=true&loc=Local")
	if err != nil {
		logger.Fatalf("InitDB sqlx.Connect", zap.Error(err))
	}

	// 连接池参数
	db.SetMaxOpenConns(maxOpenConn)
	db.SetMaxIdleConns(maxIdleConn)
	// db.SetConnMaxLifetime(time.Second * 30)
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// 会话级超时，降低锁等待时长
	if _, err := db.Exec("SET SESSION innodb_lock_wait_timeout = ?", 5); err != nil {
		logger.Warn("SET innodb_lock_wait_timeout failed", zap.Error(err))
	}

	err = db.Ping()
	if err != nil {
		logger.Fatalf("InitDB failed:", zap.Error(err))
	}

	return db
}

// 初始化slave db
func InitSlaveDB(dsn string, maxIdleConn, maxOpenConn int) *sqlx.DB {
	db, err := sqlx.Connect("mysql", dsn+"&parseTime=true&loc=Local")
	if err != nil {
		logger.Fatalf("InitSlaveDB  sqlx.Connect failed:", zap.Error(err))
	}

	// 连接池参数
	db.SetMaxOpenConns(maxOpenConn)
	db.SetMaxIdleConns(maxIdleConn)
	// db.SetConnMaxLifetime(time.Second * 30)
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// 会话级超时，降低锁等待时长（从库通常不事务写，但保持一致；失败仅记录告警）
	if _, err := db.Exec("SET SESSION innodb_lock_wait_timeout = ?", 5); err != nil {
		logger.Warn("SET innodb_lock_wait_timeout (slave) failed", zap.Error(err))
	}

	err = db.Ping()
	if err != nil {
		logger.Fatalf("InitSlaveDB failed:", zap.Error(err))
	}

	return db
}

// 初始化Redis哨兵连接
func InitRedis(dsn string, psd string, db int) *redis.Client {
	reddb := redis.NewClient(&redis.Options{
		Network:      "tcp",
		Addr:         dsn,
		Username:     "",
		DB:           db,
		Password:     psd,
		DialTimeout:  10 * time.Second, // 设置连接超时
		ReadTimeout:  10 * time.Second, // 设置读取超时
		WriteTimeout: 5 * time.Second,  // 设置写入超时
		PoolSize:     500,              // 连接池最大socket连接数，默认为5倍CPU数， 5 * runtime.NumCPU
		MinIdleConns: 100,              // 在启动阶段创建指定数量的Idle连接，并长期维持idle状态的连接数不少于指定数量；。
		PoolTimeout:  11 * time.Second, // 当所有连接都处在繁忙状态时，客户端等待可用连接的最大等待时长，默认为读超时+1秒。
		MaxRetries:   1,                // 命令执行失败时，最多重试多少次，默认为0即不重试
		IdleTimeout:  2 * time.Minute,  // 闲置超时，默认5分钟，-1表示取消闲置超时检查
	})
	return reddb
}

// 初始化redis master
func InitRedisCluster(dsn []string, pwd string) *redis.ClusterClient {
	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          dsn,              // 集群节点地址，理论上只要填一个可用的节点客户端就可以自动获取到集群的所有节点信息。但是最好多填一些节点以增加容灾能力，因为只填一个节点的话，如果这个节点出现了异常情况，则Go应用程序在启动过程中无法获取到集群信息。
		Password:       pwd,              // 密码
		MaxRedirects:   8,                // 当遇到网络错误或者MOVED/ASK重定向命令时，最多重试几次，默认8
		ReadOnly:       true,             // 只含读操作的命令的"节点选择策略"。默认都是false，即只能在主节点上执行。 置为true则允许在从节点上执行只含读操作的命令
		RouteByLatency: false,            // 默认false。 置为true则ReadOnly自动置为true,表示在处理只读命令时，可以在一个slot对应的主节点和所有从节点中选取Ping()的响应时长最短的一个节点来读数据
		RouteRandomly:  true,             // 默认false。置为true则ReadOnly自动置为true,表示在处理只读命令时，可以在一个slot对应的主节点和所有从节点中随机挑选一个节点来读数据
		DialTimeout:    10 * time.Second, // 设置连接超时
		ReadTimeout:    10 * time.Second, // 设置读取超时
		WriteTimeout:   5 * time.Second,  // 设置写入超时
		PoolSize:       500,              // 连接池最大socket连接数，默认为5倍CPU数， 5 * runtime.NumCPU
		MinIdleConns:   100,              // 在启动阶段创建指定数量的Idle连接，并长期维持idle状态的连接数不少于指定数量；。
		PoolTimeout:    11 * time.Second, // 当所有连接都处在繁忙状态时，客户端等待可用连接的最大等待时长，默认为读超时+1秒。
		MaxRetries:     1,                // 命令执行失败时，最多重试多少次，默认为0即不重试
		IdleTimeout:    2 * time.Minute,  // 闲置超时，默认5分钟，-1表示取消闲置超时检查
	})

	_, err := clusterClient.Ping().Result()
	if err != nil {
		logger.Fatalf("initRedisCluster failed:", zap.Error(err))
	}

	return clusterClient
}

// 初始化Redis哨兵连接
func InitRedisSentinel(dsn []string, psd, name string, db int) *redis.Client {
	reddb := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    name,
		SentinelAddrs: dsn,
		Password:      psd, // no password set
		DB:            db,  // use default DB
		DialTimeout:   10 * time.Second,
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  30 * time.Second,
		PoolSize:      100,
		PoolTimeout:   30 * time.Second,
		MaxRetries:    2,
		IdleTimeout:   5 * time.Minute,
	})
	_, err := reddb.Ping().Result()
	if err != nil {
		logger.Fatalf("initRedisSentinel failed:", zap.Error(err))

	}

	return reddb
}
