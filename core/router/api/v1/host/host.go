package host

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/labulaka521/crocodile/common/log"
	"github.com/labulaka521/crocodile/common/utils"
	"github.com/labulaka521/crocodile/core/config"
	"github.com/labulaka521/crocodile/core/model"
	"github.com/labulaka521/crocodile/core/utils/define"
	"github.com/labulaka521/crocodile/core/utils/resp"
	"go.uber.org/zap"
)

// GetHost return all registry gost
// @Summary get all hosts
// @Tags Host
// @Description get all registry host
// @Param offset query int false "Offset"
// @Param limit query int false "Limit"
// @Produce json
// @Success 200 {object} resp.Response
// @Router /api/v1/host [get]
// @Security ApiKeyAuth
func GetHost(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(),
		config.CoreConf.Server.DB.MaxQueryTime.Duration)
	defer cancel()
	var (
		q   define.Query
		err error
	)

	err = c.BindQuery(&q)
	if err != nil {
		log.Error("BindQuery offset failed", zap.Error(err))
	}

	if q.Limit == 0 {
		q.Limit = define.DefaultLimit
	}

	hosts, err := model.GetHosts(ctx, q.Offset, q.Limit)

	if err != nil {
		log.Error("GetHost failed", zap.String("error", err.Error()))
		resp.JSON(c, resp.ErrInternalServer, nil)
		return
	}

	resp.JSON(c, resp.Success, hosts)
}

// ChangeHostState stop host worker
// @Summary stop host worker
// @Tags Host
// @Description stop host worker
// @Param StopHost body define.GetID true "ID"
// @Produce json
// @Success 200 {object} resp.Response
// @Router /api/v1/host/stop [put]
// @Security ApiKeyAuth
func ChangeHostState(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(),
		config.CoreConf.Server.DB.MaxQueryTime.Duration)
	defer cancel()
	hosttask := define.GetID{}
	err := c.ShouldBindJSON(&hosttask)
	if err != nil {
		resp.JSON(c, resp.ErrBadRequest, nil)
		return
	}
	if utils.CheckID(hosttask.ID) != nil {
		resp.JSON(c, resp.ErrBadRequest, nil)
		return
	}
	host, err := model.GetHostByID(ctx, hosttask.ID)
	if err != nil {
		resp.JSON(c, resp.ErrInternalServer, nil)
		return
	}

	err = model.StopHost(ctx, hosttask.ID, host.Stop^1)
	if err != nil {
		resp.JSON(c, resp.ErrInternalServer, nil)
		return
	}
	resp.JSON(c, resp.Success, nil)
}

// DeleteHost delete host
// @Summary delete host
// @Tags Host
// @Description delete host
// @Param StopHost body define.GetID true "ID"
// @Produce json
// @Success 200 {object} resp.Response
// @Router /api/v1/host [delete]
// @Security ApiKeyAuth
func DeleteHost(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(),
		config.CoreConf.Server.DB.MaxQueryTime.Duration)
	defer cancel()
	gethost := define.GetID{}
	err := c.ShouldBindJSON(&gethost)
	if err != nil {
		resp.JSON(c, resp.ErrBadRequest, nil)
		return
	}
	if utils.CheckID(gethost.ID) != nil {
		resp.JSON(c, resp.ErrBadRequest, nil)
		return
	}

	hostgroups, err := model.GetHostGroups(ctx, 0, 0)
	if err != nil {
		resp.JSON(c, resp.ErrInternalServer, nil)
		return
	}
	for _, hostgroup := range hostgroups {
		for _, hid := range hostgroup.HostsID {
			if gethost.ID == hid {
				resp.JSON(c, resp.ErrDelHostUseByOtherHG, nil)
			}
		}
	}

	err = model.DeleteHost(ctx, gethost.ID)
	if err != nil {
		log.Error("model.DeleteHost", zap.String("error", err.Error()))
		resp.JSON(c, resp.ErrInternalServer, nil)
		return
	}
	resp.JSON(c, resp.Success, nil)
}

