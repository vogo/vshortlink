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

	"github.com/redis/go-redis/v9"
	"github.com/vogo/vogo/vlog"
	"github.com/vogo/vogo/vos"
	"github.com/vogo/vshortlink/cores"
	"github.com/vogo/vshortlink/gormx"
	"github.com/vogo/vshortlink/redisx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ShortLinkServer short link server，using gormx's repo and redisx's pool and cache
type ShortLinkServer struct {
	*cores.ShortLinkService
}

// NewShortLinkServer create short link server
func NewShortLinkServer(
	db *gorm.DB,
	redisClient *redis.Client,
	opts ...cores.ServiceOption,
) *ShortLinkServer {
	repo := gormx.NewGormShortLinkRepository(db)

	cache := redisx.NewRedisShortLinkCache(redisClient)

	pool := redisx.NewRedisShortCodePool(redisClient)

	coreService := cores.NewShortLinkService(repo, cache, pool, opts...)

	return &ShortLinkServer{
		ShortLinkService: coreService,
	}
}

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

	service := NewShortLinkServer(db, redisClient,
		cores.WithBatchGenerateSize(batchGenerateSize),
		cores.WithMaxCodeLength(maxCodeLength),
		cores.WithAuthToken(authToken))

	http.HandleFunc("/", service.HttpHandle)

	serverAddr := fmt.Sprintf(":%s", serverPort)
	vlog.Infof("server listen at %s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
