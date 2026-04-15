/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vogo/vogo/vlog"
	"github.com/vogo/vogo/vos"
	"github.com/vogo/vshortlink/cores"
	"github.com/vogo/vshortlink/ext/gormx"
	"github.com/vogo/vshortlink/ext/redisx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	mysqlHost := vos.GetEnvStr("MYSQL_HOST", "localhost")
	mysqlPort := vos.GetEnvStr("MYSQL_PORT", "3306")
	mysqlUser := vos.GetEnvStr("MYSQL_USER", "root")
	mysqlPassword := vos.GetEnvStr("MYSQL_PASSWORD", "")
	mysqlDatabase := vos.GetEnvStr("MYSQL_DATABASE", "vshortlink")

	redisHost := vos.GetEnvStr("REDIS_HOST", "localhost")
	redisPort := vos.GetEnvStr("REDIS_PORT", "6379")
	redisPassword := vos.GetEnvStr("REDIS_PASSWORD", "")
	redisDB := vos.GetEnvInt("REDIS_DB", 0)

	serverPort := vos.GetEnvStr("SERVER_PORT", "8080")
	batchGenerateSize := vos.GetEnvInt64("BATCH_GENERATE_SIZE", 100)
	maxCodeLength := vos.GetEnvInt("MAX_CODE_LENGTH", 6)

	authToken := vos.GetEnvStr("AUTH_TOKEN", "")

	statsRetentionDays := vos.GetEnvInt("STATS_RETENTION_DAYS", 7)
	statsFlushSeconds := vos.GetEnvInt("STATS_FLUSH_INTERVAL_SECONDS", 1)
	statsBufferSize := vos.GetEnvInt("STATS_BUFFER_SIZE", 10000)
	statsTimezone := vos.GetEnvStr("STATS_TIMEZONE", "UTC")

	statsLoc, err := time.LoadLocation(statsTimezone)
	if err != nil {
		vlog.Fatalf("invalid STATS_TIMEZONE %q: %v", statsTimezone, err)
	}

	mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDatabase)

	db, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		vlog.Fatalf("failed to connect to mysql: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: redisPassword,
		DB:       redisDB,
	})

	ctx := context.Background()
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		vlog.Fatalf("failed to ping redis: %v", err)
	}

	dbFunc := func() *gorm.DB {
		return db
	}
	repo := gormx.NewGormShortLinkRepository(dbFunc)
	cache := redisx.NewRedisShortLinkCache(redisClient)
	pool := redisx.NewRedisShortCodePool(redisClient)
	stats := redisx.NewRedisShortLinkStats(redisClient,
		redisx.WithStatsRetentionDays(statsRetentionDays))

	service := cores.NewShortLinkService(repo, cache, pool,
		cores.WithBatchGenerateSize(batchGenerateSize),
		cores.WithMaxCodeLength(maxCodeLength),
		cores.WithAuthToken(authToken),
		cores.WithStats(stats),
		cores.WithStatsTimezone(statsLoc),
		cores.WithStatsRetentionDays(statsRetentionDays),
		cores.WithStatsFlushInterval(time.Duration(statsFlushSeconds)*time.Second),
		cores.WithStatsBufferSize(statsBufferSize))

	defer service.Close()

	http.HandleFunc("/", service.HttpHandle)

	serverAddr := fmt.Sprintf(":%s", serverPort)
	vlog.Infof("server listen at %s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
