# go-rblx-asset-scraper

## Introduction

This repository contains a collection of scripts and deployments for scraping ROBLOX's public game assets. Currently about 10% of ROBLOX scripts (a small subset of the entire asset collection) have been scraped, equating to about 500 GB compressed.

ROBLOX distributes links to assets in their CDN through the Asset Delivery API. The Asset Delivery API lets you query up to 256 different ID's at a time (a bizarre restraint). Hence, if you want to scrape a sparse subset of their 10 billion assets in a reasonable amount of time, you have to make _many_ requests to the Asset Delivery API.

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
  System_Ext(rblx, "ROBLOX Asset Delivery Service", "Query ROBLOX assets by ID, and distribute CDN links on demand")
  SystemDb_Ext(cdn, "ROBLOX Asset CDN", "Direct access to contents of ROBLOX game assets")
  SystemDb_Ext(s3, "Wasabi S3", "Provides S3-compatible storage over the internet")

  Rel(orchestrator, do, "dispatches to")
  Rel(do, sync, "dispatches to")
  Rel(sync, proxy, "query asset IDs through proxy")
  Rel(proxy, rblx, "proxied requests")
  Rel(sync, cdn, "download asset contents from CDN")
  BiRel(rblx, cdn, "uses")
  Rel(sync, s3, "stores assets in")
```
