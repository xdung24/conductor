package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/models"
)

func (h *Handler) maintenanceStore(c *gin.Context) *models.MaintenanceStore {
	return models.NewMaintenanceStore(h.userDB(c))
}

// MaintenanceList renders the maintenance windows management page.
func (h *Handler) MaintenanceList(c *gin.Context) {
	windows, _ := h.maintenanceStore(c).List()
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "maintenance_list.html", h.pageData(c, gin.H{
		"Windows": windows,
		"Flash":   flash,
	}))
}

// MaintenanceNew renders the create form.
func (h *Handler) MaintenanceNew(c *gin.Context) {
	monitors, _ := h.monitorStore(c).List()
	c.HTML(http.StatusOK, "maintenance_form.html", h.pageData(c, gin.H{
		"Window":           &models.MaintenanceWindow{Active: true},
		"IsNew":            true,
		"AllMonitors":      monitors,
		"LinkedMonitorIDs": map[int64]bool{},
		"Error":            "",
	}))
}

// MaintenanceCreate handles the create form submission.
func (h *Handler) MaintenanceCreate(c *gin.Context) {
	window, monitorIDs, err := maintenanceFromForm(c)
	if err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusBadRequest, "maintenance_form.html", gin.H{
			"Window": window, "IsNew": true, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}

	mStore := h.maintenanceStore(c)
	id, err := mStore.Create(window)
	if err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusInternalServerError, "maintenance_form.html", gin.H{
			"Window": window, "IsNew": true, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}

	_ = mStore.SetMonitors(id, monitorIDs)
	c.SetCookie("sm_flash", "Maintenance window created", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/maintenance")
}

// MaintenanceEdit renders the edit form.
func (h *Handler) MaintenanceEdit(c *gin.Context) {
	window, ok := h.getMaintWindow(c)
	if !ok {
		return
	}
	mStore := h.maintenanceStore(c)
	monitors, _ := h.monitorStore(c).List()
	linkedIDs, _ := mStore.ListMonitorIDs(window.ID)
	linked := make(map[int64]bool, len(linkedIDs))
	for _, id := range linkedIDs {
		linked[id] = true
	}
	c.HTML(http.StatusOK, "maintenance_form.html", h.pageData(c, gin.H{
		"Window":           window,
		"IsNew":            false,
		"AllMonitors":      monitors,
		"LinkedMonitorIDs": linked,
		"Error":            "",
	}))
}

// MaintenanceUpdate handles the edit form submission.
func (h *Handler) MaintenanceUpdate(c *gin.Context) {
	existing, ok := h.getMaintWindow(c)
	if !ok {
		return
	}
	updated, monitorIDs, err := maintenanceFromForm(c)
	if err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusBadRequest, "maintenance_form.html", gin.H{
			"Window": existing, "IsNew": false, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}
	updated.ID = existing.ID

	mStore := h.maintenanceStore(c)
	if err := mStore.Update(updated); err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusInternalServerError, "maintenance_form.html", gin.H{
			"Window": updated, "IsNew": false, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}
	_ = mStore.SetMonitors(updated.ID, monitorIDs)
	c.SetCookie("sm_flash", "Maintenance window updated", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/maintenance")
}

// MaintenanceDelete removes a maintenance window.
func (h *Handler) MaintenanceDelete(c *gin.Context) {
	window, ok := h.getMaintWindow(c)
	if !ok {
		return
	}
	if err := h.maintenanceStore(c).Delete(window.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	c.SetCookie("sm_flash", "Maintenance window deleted", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/maintenance")
}

func (h *Handler) getMaintWindow(c *gin.Context) (*models.MaintenanceWindow, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "invalid window id"})
		return nil, false
	}
	w, err := h.maintenanceStore(c).Get(id)
	if err != nil || w == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "maintenance window not found"})
		return nil, false
	}
	return w, true
}

// maintenanceFromForm parses a maintenance window from a POST form.
func maintenanceFromForm(c *gin.Context) (*models.MaintenanceWindow, []int64, error) {
	name := c.PostForm("name")
	startStr := c.PostForm("start_time")
	endStr := c.PostForm("end_time")
	active := c.PostForm("active") != "off"

	w := &models.MaintenanceWindow{Name: name, Active: active}

	if name == "" {
		return w, nil, &formError{"name is required"}
	}
	if startStr == "" || endStr == "" {
		return w, nil, &formError{"start time and end time are required"}
	}

	// Accept both datetime-local format (T separator) and space-separated.
	const layout1 = "2006-01-02T15:04"
	const layout2 = "2006-01-02 15:04"
	start, err := time.ParseInLocation(layout1, startStr, time.Local)
	if err != nil {
		start, err = time.ParseInLocation(layout2, startStr, time.Local)
	}
	if err != nil {
		return w, nil, &formError{"invalid start time format (use YYYY-MM-DDTHH:MM)"}
	}

	end, err := time.ParseInLocation(layout1, endStr, time.Local)
	if err != nil {
		end, err = time.ParseInLocation(layout2, endStr, time.Local)
	}
	if err != nil {
		return w, nil, &formError{"invalid end time format (use YYYY-MM-DDTHH:MM)"}
	}

	if !end.After(start) {
		return w, nil, &formError{"end time must be after start time"}
	}

	w.StartTime = start.UTC()
	w.EndTime = end.UTC()

	var monitorIDs []int64
	for _, v := range c.PostFormArray("monitors") {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			monitorIDs = append(monitorIDs, id)
		}
	}
	return w, monitorIDs, nil
}
