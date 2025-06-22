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

package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/vogo/vshortlink/cores"
	"github.com/vogo/vshortlink/gormx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormExample demonstrates how to use the GORM-based ShortLinkService
func GormExample() {
	// Connect to MySQL database
	dsn := "user:password@tcp(127.0.0.1:3306)/vshortlink?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}

	// Create a new GORM-based ShortLinkService
	// batchGenerateSize: 100, maxCodeLength: 6
	service := gormx.NewGormShortLinkService(db, cores.WithBatchGenerateSize(100), cores.WithMaxCodeLength(6))

	// Create a context
	ctx := context.Background()

	// Create a short link that expires in 1 hour
	link, err := service.ShortLinkService.Create(ctx, "https://example.com", 4, time.Now().Add(time.Hour))
	if err != nil {
		panic(fmt.Sprintf("failed to create short link: %v", err))
	}

	fmt.Printf("Created short link: %s with code: %s\n", link.Link, link.Code)

	// Get the short link by code
	retrievedLink, err := service.Repo.GetByCode(ctx, link.Code)
	if err != nil {
		panic(fmt.Sprintf("failed to get short link: %v", err))
	}

	fmt.Printf("Retrieved short link: %s with code: %s\n", retrievedLink.Link, retrievedLink.Code)

	// Update the expiration time
	retrievedLink.Expire = time.Now().Add(2 * time.Hour)
	err = service.Repo.Update(ctx, retrievedLink)
	if err != nil {
		panic(fmt.Sprintf("failed to update short link: %v", err))
	}

	fmt.Println("Updated short link expiration time")

	// Create a short link that expires immediately for testing expiration
	expiredLink, err := service.ShortLinkService.Create(ctx, "https://expired-example.com", 4, time.Now().Add(-time.Second))
	if err != nil {
		panic(fmt.Sprintf("failed to create expired short link: %v", err))
	}

	fmt.Printf("Created expired short link with code: %s\n", expiredLink.Code)

	// Process expired active links
	service.ShortLinkService.ExpireActives()
	fmt.Println("Processed expired active links")

	// Try to get the expired link
	_, err = service.Repo.GetByCode(ctx, expiredLink.Code)
	if err != nil {
		fmt.Printf("Expected error getting expired link: %v\n", err)
	}

	// Recycle expired links
	service.ShortLinkService.RecycleExpires()
	fmt.Println("Recycled expired links")

	// Create a new link and check if the recycled code is reused
	newLink, err := service.ShortLinkService.Create(ctx, "https://new-example.com", 4, time.Now().Add(time.Hour))
	if err != nil {
		panic(fmt.Sprintf("failed to create new short link: %v", err))
	}

	fmt.Printf("Created new short link with code: %s\n", newLink.Code)

	// Check if the code was recycled
	if newLink.Code == expiredLink.Code {
		fmt.Println("Successfully recycled the expired short code!")
	}
}
