package appstate

import "trading_system/internal/models"

var (
	globalTickGen *models.TickGenerator
)

func SetGlobalTickGenerator(gen *models.TickGenerator) {
	globalTickGen = gen
}

func GetGlobalTickGenerator() *models.TickGenerator {
	return globalTickGen
}
