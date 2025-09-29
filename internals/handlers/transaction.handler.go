package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prospera/internals/models"
	"github.com/prospera/internals/pkg"
	"github.com/prospera/internals/repositories"
	"github.com/prospera/internals/utils"
	"github.com/redis/go-redis/v9"
)

type TransactionHandler struct {
	repo     *repositories.TransactionRepository
	repoAuth *repositories.Auth
	rdb      *redis.Client
}

func NewTransactionHandler(repo *repositories.TransactionRepository, rdb *redis.Client, repoAuth *repositories.Auth) *TransactionHandler {
	return &TransactionHandler{repo: repo, rdb: rdb, repoAuth: repoAuth}
}

// Create Transactions
func (h *TransactionHandler) CreateTransaction(ctx *gin.Context) {
	uid, err := utils.GetUserIDFromJWT(ctx)
	if err != nil {
		utils.HandleError(ctx, http.StatusUnauthorized, "Unauthorized", "invalid token", err)
		return
	}

	var req models.TransactionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.HandleError(ctx, http.StatusBadRequest, "Bad Request", "invalid payload", err)
		return
	}

	// Check PIN only for transfer type
	if req.Type == "transfer" {
		// Ambil PIN user dari DB
		storedPIN, err := h.repoAuth.VerifyUserPIN(ctx.Request.Context(), uid)
		if err != nil {
			utils.HandleError(ctx, http.StatusInternalServerError, "Internal Server Error", "failed to fetch user pin", err)
			return
		}

		// Bandingkan PIN yang dikirim dengan hash
		hashConfig := pkg.NewHashConfig()
		hashConfig.UseRecommended()

		valid, err := hashConfig.ComparePasswordAndHash(req.PIN, storedPIN)
		if err != nil {
			utils.HandleError(ctx, http.StatusInternalServerError, "Internal Server Error", "failed to verify pin", err)
			return
		}
		if !valid {
			utils.HandleError(ctx, http.StatusForbidden, "Forbidden", "invalid PIN", fmt.Errorf("invalid PIN"))
			return
		}
	}

	// Buat transaksi
	if err := h.repo.CreateTransaction(ctx.Request.Context(), &req, uid); err != nil {
		utils.HandleError(ctx, http.StatusInternalServerError, "Internal Server Error", "failed to create transaction", err)
		return
	}

	if req.Type == "transfer" {
		message := fmt.Sprintf("Kamu menerima transfer Rp%d.00 dari user %d", req.Amount, uid)
		pkg.WebSocketHub.SendToUser(*req.ReceiverAccountID, message)
	} else {
		message := fmt.Sprintf("Kamu menerima transfer Rp%d.00 dari user %d", req.Amount, uid)
		pkg.WebSocketHub.SendToUser(uid, message)
	}

	var redisKey1 = fmt.Sprintf("Prospera-Summary-daily-%d", uid)
	if err := utils.InvalidateCache(ctx, h.rdb, redisKey1); err != nil {
		log.Println("Failed invalidate cache:", err)
	}
	var redisKey2 = fmt.Sprintf("Prospera-Summary-weekly-%d", uid)
	if err := utils.InvalidateCache(ctx, h.rdb, redisKey2); err != nil {
		log.Println("Failed invalidate cache:", err)
	}
	var redisKey3 = fmt.Sprintf("Prospera-Balance-%d", uid)
	if err := utils.InvalidateCache(ctx, h.rdb, redisKey3); err != nil {
		log.Println("Failed invalidate cache:", err)
	}
	var redisKey4 = fmt.Sprintf("Prospera-HistoryTransaction-1-%d", uid)
	if err := utils.InvalidateCache(ctx, h.rdb, redisKey4); err != nil {
		log.Println("Failed invalidate cache:", err)
	}

	if req.Type == "transfer" {
		var redisKey2 = fmt.Sprintf("Prospera-Summary-daily-%d", *req.ReceiverAccountID)
		if err := utils.InvalidateCache(ctx, h.rdb, redisKey2); err != nil {
			log.Println("Failed invalidate cache:", err)
		}
		var redisKey1 = fmt.Sprintf("Prospera-Summary-weekly-%d", *req.ReceiverAccountID)
		if err := utils.InvalidateCache(ctx, h.rdb, redisKey1); err != nil {
			log.Println("Failed invalidate cache:", err)
		}
		var redisKey3 = fmt.Sprintf("Prospera-Balance-%d", *req.ReceiverAccountID)
		if err := utils.InvalidateCache(ctx, h.rdb, redisKey3); err != nil {
			log.Println("Failed invalidate cache:", err)
		}
		var redisKey4 = fmt.Sprintf("Prospera-HistoryTransaction-1-%d", *req.ReceiverAccountID)
		if err := utils.InvalidateCache(ctx, h.rdb, redisKey4); err != nil {
			log.Println("Failed invalidate cache:", err)
		}
	}

	ctx.JSON(http.StatusOK, models.Response[any]{
		Success: true,
		Message: "Transaction Success",
	})
}
