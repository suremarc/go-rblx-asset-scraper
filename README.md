# go-rblx-asset-scraper

## Introduction

This repository contains a collection of scripts and deployments for scraping ROBLOX's public game assets.

```mermaid
C4Context
  title ROBLOX Asset Scraper - high level overview
  Boundary(scraper, "scraper") {
    System(sync, "scraper/sync", "A DigitalOcean function that syncs ROBLOX assets to Wasabi S3")
    SystemQueue_Ext(do, "DigitalOcean Functions", "Platform for deploying serverless applications")
    System(orchestrator, "orchestrator", "A Docker deployment that kicks off sync jobs")
    SystemDb(postgres, "PostgreSQL Job Log", "Stores records of completed/failed sync jobs in PostgreSQL")
    Rel(orchestrator, postgres, "log results in")
  }
  System_Ext(proxy, "BrightData Data Center Proxy", "Rotate requests among data center IPs to get around rate limits")
  System_Ext(rblx, "ROBLOX Asset Delivery Service", "Access to ROBLOX's asset CDN for distribution of game assets")
  SystemDb_Ext(s3, "Wasabi S3", "Provides S3-compatible storage over the internet")

  Rel(orchestrator, do, "dispatches to")
  Rel(do, sync, "dispatches to")
  Rel(sync, proxy, "query asset IDs through proxy")
  Rel(proxy, rblx, "proxied requests")
  Rel(sync, rblx, "fetch asset contents by ID")
  Rel(sync, s3, "stores assets in")
```
