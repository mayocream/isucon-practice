package main

import "github.com/gofiber/fiber/v2"

func routeRegister(s *fiber.App) {
    // Initialize
    s.Post("/initialize", initialize)

    // Chair Handler
    s.Post("/api/chair", postChair)
    s.Get("/api/chair/search", searchChairs)
    s.Get("/api/chair/low_priced", getLowPricedChair)
    s.Get("/api/chair/search/condition", getChairSearchCondition)
    s.Get("/api/chair/:id", getChairDetail)
    s.Post("/api/chair/buy/:id", buyChair)

    // Estate Handler
    s.Post("/api/estate", postEstate)
    s.Get("/api/estate/search", searchEstates)
    s.Get("/api/estate/low_priced", getLowPricedEstate)
    s.Post("/api/estate/req_doc/:id", postEstateRequestDocument)
    s.Post("/api/estate/nazotte", searchEstateNazotte)
    s.Get("/api/estate/search/condition", getEstateSearchCondition)
    s.Get("/api/estate/:id", getEstateDetail)
    s.Get("/api/recommended_estate/:id", searchRecommendedEstateWithChair)
}