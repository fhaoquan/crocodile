package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/labulaka521/crocodile/common/db"
	"github.com/labulaka521/crocodile/common/log"
	"github.com/labulaka521/crocodile/common/utils"
	pb "github.com/labulaka521/crocodile/core/proto"
	"github.com/labulaka521/crocodile/core/utils/define"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	maxWorkerTTL int64 = 20 // defaultHearbeatInterval = 15
)

// RegistryNewHost refistry new host
func RegistryNewHost(ctx context.Context, req *pb.RegistryReq) (string, error) {
	hostsql := `INSERT INTO crocodile_host 
					(id,
					hostname,
					addr,
					weight,
					version,
					lastUpdateTimeUnix,
					remark
				)
 			  	VALUES
					(?,?,?,?,?,?,?)`
	addr := fmt.Sprintf("%s:%d", req.Ip, req.Port)
	hosts, err := getHosts(ctx, addr, nil, 0, 0)
	if err != nil {
		return "", err
	}
	if len(hosts) == 1 {
		log.Info("Addr Already Registry", zap.String("addr", addr))
		return "", nil
	}
	conn, err := db.GetConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "db.GetConn")
	}
	defer conn.Close()
	stmt, err := conn.PrepareContext(ctx, hostsql)
	if err != nil {
		return "", errors.Wrap(err, "conn.PrepareContext")
	}
	defer stmt.Close()
	id := utils.GetID()
	_, err = stmt.ExecContext(ctx,
		id,
		req.Hostname,
		addr,
		req.Weight,
		req.Version,
		time.Now().Unix(),
		req.Remark,
	)
	if err != nil {
		return "", errors.Wrap(err, "stmt.ExecContext")
	}
	log.Info("New Client Registry ", zap.String("addr", addr))
	return id, nil
}

// UpdateHostHearbeat update host last recv hearbeat time
func UpdateHostHearbeat(ctx context.Context, ip string, port int32, runningtasks []string) error {
	updatesql := `UPDATE crocodile_host set lastUpdateTimeUnix=?,runningTasks=? WHERE addr=?`
	conn, err := db.GetConn(ctx)
	if err != nil {
		return errors.Wrap(err, "db.GetConn")
	}
	defer conn.Close()
	stmt, err := conn.PrepareContext(ctx, updatesql)
	if err != nil {
		return errors.Wrap(err, "conn.PrepareContext")
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(ctx,
		time.Now().Unix(),
		strings.Join(runningtasks, ","),
		fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return errors.Wrap(err, "stmt.ExecContext")
	}
	return nil
}

// get host by addr or id
func getHosts(ctx context.Context, addr string, ids []string, offset, limit int) ([]*define.Host, error) {
	getsql := `SELECT 
					id,
					addr,
					hostname,
					runningTasks,
					weight,
					stop,
					version,
					lastUpdateTimeUnix 
			   FROM 
					crocodile_host`
	args := []interface{}{}
	if addr != "" {
		getsql += " WHERE addr=?"
		args = append(args, addr)
	}

	if len(ids) > 0 {
		var querys = []string{}
		for _, id := range ids {
			querys = append(querys, "id=?")
			args = append(args, id)
		}
		getsql += " WHERE " + strings.Join(querys, " OR ")

	}
	if limit > 0 {
		getsql += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	conn, err := db.GetConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "db.GetConn")
	}
	defer conn.Close()
	stmt, err := conn.PrepareContext(ctx, getsql)
	if err != nil {
		return nil, errors.Wrap(err, "conn.PrepareContext")
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, errors.Wrap(err, "stmt.QueryContext")
	}

	hosts := []*define.Host{}
	for rows.Next() {
		var (
			h     define.Host
			rtask string
		)

		err := rows.Scan(&h.ID, &h.Addr, &h.HostName, &rtask, &h.Weight, &h.Stop, &h.Version, &h.LastUpdateTimeUnix)
		if err != nil {
			log.Error("Scan failed", zap.Error(err))
			continue
		}
		h.RunningTasks = []string{}
		if rtask != "" {
			h.RunningTasks = append(h.RunningTasks, strings.Split(rtask, ",")...)
		}
		if h.LastUpdateTimeUnix+maxWorkerTTL > time.Now().Unix() {
			h.Online = 1
		}
		h.LastUpdateTime = utils.UnixToStr(h.LastUpdateTimeUnix)
		hosts = append(hosts, &h)
	}
	return hosts, nil
}

// GetHosts get all hosts
func GetHosts(ctx context.Context, offset, limit int) ([]*define.Host, error) {
	return getHosts(ctx, "", nil, offset, limit)
}

// GetHostByAddr get host by addr
func GetHostByAddr(ctx context.Context, addr string) (*define.Host, error) {
	hosts, err := getHosts(ctx, addr, nil, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(hosts) != 1 {
		return nil, errors.New("can not find hostid")
	}
	return hosts[0], nil
}

// ExistAddr check already exist
func ExistAddr(ctx context.Context, addr string) (*define.Host, bool, error) {
	hosts, err := getHosts(ctx, addr, nil, 0, 0)
	if err != nil {
		return nil, false, err
	}
	if len(hosts) != 1 {
		return nil, false, nil
	}
	return hosts[0], true, nil
}

// GetHostByID get host by hostid
func GetHostByID(ctx context.Context, id string) (*define.Host, error) {
	hosts, err := getHosts(ctx, "", []string{id}, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(hosts) != 1 {
		return nil, errors.New("can not find hostid")
	}
	return hosts[0], nil
}

// GetHostByIDS get hosts by hostids
func GetHostByIDS(ctx context.Context, ids []string) ([]*define.Host, error) {
	hosts, err := getHosts(ctx, "", ids, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(hosts) != 1 {
		return nil, errors.New("can not find hostid")
	}
	return hosts, nil
}

// StopHost will stop run worker in hostid
func StopHost(ctx context.Context, hostid string, stop int) error {
	stopsql := `UPDATE crocodile_host SET stop=?`
	conn, err := db.GetConn(ctx)
	if err != nil {
		return errors.Wrap(err, "db.GetConn")
	}
	defer conn.Close()
	stmt, err := conn.PrepareContext(ctx, stopsql)
	if err != nil {
		return errors.Wrap(err, "conn.PrepareContext")
	}
	_, err = stmt.ExecContext(ctx, stop)
	if err != nil {
		return errors.Wrap(err, "stmt.ExecContext")
	}
	return nil
}

// DeleteHost will delete host
func DeleteHost(ctx context.Context, hostid string) error {
	err := StopHost(ctx, hostid, 0)
	if err != nil {
		return errors.Wrap(err, "StopHost")
	}
	deletehostsql := `DELETE FROM crocodile_host WHERE id=?`
	conn, err := db.GetConn(ctx)
	if err != nil {
		return errors.Wrap(err, "db.GetConn")
	}
	defer conn.Close()
	stmt, err := conn.PrepareContext(ctx, deletehostsql)
	if err != nil {
		return errors.Wrap(err, "conn.PrepareContext")
	}
	_, err = stmt.ExecContext(ctx, hostid)
	if err != nil {
		return errors.Wrap(err, "stmt.ExecContext")
	}
	return nil
}

// delete from slice
func deletefromslice(deleteid string, ids []string) ([]string, bool) {
	var existid = -1
	for index, id := range ids {
		if id == deleteid {
			existid = index
			break
		}
	}
	if existid == -1 {
		// no found delete id
		return ids, false
	}
	ids = append(ids[:existid], ids[existid+1:]...)
	return ids, true
}

