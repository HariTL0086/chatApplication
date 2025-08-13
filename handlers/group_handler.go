package handlers

import (
	"Chat_App/internal/models"
	"Chat_App/internal/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

type GroupHandler struct {
	groupService *services.GroupService
}

func NewGroupHandler(groupService *services.GroupService) *GroupHandler {
	return &GroupHandler{
		groupService: groupService,
	}
}


func (h *GroupHandler) CreateGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	group, err := h.groupService.CreateGroup(c.Request.Context(), userUUID, &req)
	if err != nil {
		switch err {
		case services.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Group created successfully",
		"group":   group,
	})
}


func (h *GroupHandler) GetGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	group, err := h.groupService.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		switch err {
		case services.ErrGroupNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"group": group})
}


func (h *GroupHandler) GetUserGroups(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	groups, err := h.groupService.GetUserGroups(c.Request.Context(), userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}


func (h *GroupHandler) AddGroupMember(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req models.AddGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.groupService.AddGroupMember(c.Request.Context(), userUUID, groupID, &req)
	if err != nil {
		switch err {
		case services.ErrNotGroupAdmin:
			c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can add members"})
		case services.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		case services.ErrAlreadyMember:
			c.JSON(http.StatusConflict, gin.H{"error": "User is already a member"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member added successfully"})
}


func (h *GroupHandler) RemoveGroupMember(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req models.RemoveGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.groupService.RemoveGroupMember(c.Request.Context(), userUUID, groupID, &req)
	if err != nil {
		switch err {
		case services.ErrNotGroupAdmin:
			c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can remove members"})
		case services.ErrMemberNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		case services.ErrCannotRemoveAdmin:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove the last admin"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}


func (h *GroupHandler) ChangeMemberRole(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req models.ChangeMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.groupService.ChangeMemberRole(c.Request.Context(), userUUID, groupID, &req)
	if err != nil {
		switch err {
		case services.ErrNotGroupAdmin:
			c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can change roles"})
		case services.ErrMemberNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change role"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role changed successfully"})
}


func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req models.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	group, err := h.groupService.UpdateGroup(c.Request.Context(), userUUID, groupID, &req)
	if err != nil {
		switch err {
		case services.ErrNotGroupAdmin:
			c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can update group"})
		case services.ErrGroupNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Group updated successfully",
		"group":   group,
	})
}


func (h *GroupHandler) GetGroupMembers(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	members, err := h.groupService.GetGroupMembers(c.Request.Context(), groupID)
	if err != nil {
		switch err {
		case services.ErrGroupNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group members"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}


func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.groupService.DeleteGroup(c.Request.Context(), userUUID, groupID)
	if err != nil {
		switch err {
		case services.ErrNotGroupAdmin:
			c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can delete group"})
		case services.ErrGroupNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
}


func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	
	log.Printf("LeaveGroup: User %s attempting to leave group %s", userUUID.String(), groupID.String())

	err = h.groupService.LeaveGroup(c.Request.Context(), userUUID, groupID)
	if err != nil {
		log.Printf("LeaveGroup error: %v, error type: %T", err, err)
		
		
		errorStr := err.Error()
		switch {
		case errorStr == services.ErrMemberNotFound.Error():
			c.JSON(http.StatusNotFound, gin.H{"error": "You are not a member of this group"})
		case errorStr == services.ErrCannotRemoveAdmin.Error():
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot leave group as the last admin. Please transfer admin role first or delete the group."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave group: " + err.Error()})
		}
		return
	}

	log.Printf("LeaveGroup: User %s successfully left group %s", userUUID.String(), groupID.String())
	c.JSON(http.StatusOK, gin.H{"message": "Successfully left the group"})
} 