package model

type Privilege uint8

const (
	PBanned    Privilege = 0   // Logged in, but is banned
	PGuest     Privilege = 1   // Normal logged-in user
	PUser      Privilege = 10  // Normal logged-in user
	PReserved  Privilege = 15  // Normal logged-in user with reserved slot
	PEditor    Privilege = 25  // Edit access to site / resources
	PModerator Privilege = 50  // Access detailed player into & ban permissions.
	PAdmin     Privilege = 100 // Unrestricted admin
)
