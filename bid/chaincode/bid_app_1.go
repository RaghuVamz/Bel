/******************************************************************
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
******************************************************************/

///////////////////////////////////////////////////////////////////////
// Author : Mohan Venkataraman
// Purpose: Explore the Hyperledger/fabric and understand
// how to write an chain code, application/chain code boundaries
// The code is not the best as it has just hammered out in a day or two
// Feedback and updates are appreciated
///////////////////////////////////////////////////////////////////////

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	//"github.com/op/go-logging"

	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	// "github.com/errorpkg"
)

//////////////////////////////////////////////////////////////////////////////////////////////////
// The recType is a mandatory attribute. The original app was written with a single table
// in mind. The only way to know how to process a record was the 70's style 80 column punch card
// which used a record type field. The array below holds a list of valid record types.
// This could be stored on a blockchain table or an application
//////////////////////////////////////////////////////////////////////////////////////////////////
var recType = []string{"ARTINV", "USER", "BID", "AUCREQ", "POSTTRAN", "OPENAUC", "CLAUC", "XFER", "VERIFY"}

//////////////////////////////////////////////////////////////////////////////////////////////////
// The following array holds the list of tables that should be created
// The deploy/init deletes the tables and recreates them every time a deploy is invoked
//////////////////////////////////////////////////////////////////////////////////////////////////
var aucTables = []string{"UserTable", "UserCatTable", "ItemTable", "ItemCatTable", "ItemHistoryTable", "AuctionTable", "AucInitTable", "AucOpenTable", "BidTable", "TransTable"}

///////////////////////////////////////////////////////////////////////////////////////
// This creates a record of the Asset (Inventory)
// Includes Description, title, certificate of authenticity or image whatever..idea is to checkin a image and store it
// in encrypted form
// Example:
// Item { 113869, "Flower Urn on a Patio", "Liz Jardine", "10102007", "Original", "Floral", "Acrylic", "15 x 15 in", "sample_9.png","$600", "My Gallery }
///////////////////////////////////////////////////////////////////////////////////////

type ItemObject struct {
	ItemID      string
	RecType     string
	ItemDesc    string
	ItemDetail  string // Could included details such as who created the Art work if item is a Painting
	ItemType    string
	ItemSubject string
}

////////////////////////////////////////////////////////////////////////////////
// Has an item entry every time the item changes hands
////////////////////////////////////////////////////////////////////////////////
type ItemLog struct {
	ItemID       string // PRIMARY KEY
	Status       string // SECONDARY KEY - OnAuc, OnSale, NA
	AuctionedBy  string // SECONDARY KEY - Auction House ID if applicable
	RecType      string // ITEMHIS
	ItemDesc     string
	CurrentOwner string
	Date         string // Date when status changed
}

/////////////////////////////////////////////////////////////
// Create Buyer, Seller , Auction House, Authenticator
// Could establish valid UserTypes -
// AH (Auction House)
// TR (Buyer or Seller)
// AP (Appraiser)
// IN (Insurance)
// BK (bank)
// SH (Shipper)
/////////////////////////////////////////////////////////////
type UserObject struct {
	UserID    string
	RecType   string // Type = USER
	Name      string
	UserType  string // Auction House (AH), Bank (BK), Buyer or Seller (TR), Shipper (SH), Appraiser (AP)
	Address   string
	Phone     string
	Email     string
	Bank      string
	AccountNo string
	RoutingNo string
}

/////////////////////////////////////////////////////////////////////////////
// Register a request for participating in an auction
// Usually posted by a seller who owns a piece of ITEM
// The Auction house will determine when to open the item for Auction
// The Auction House may conduct an appraisal and genuineness of the item
/////////////////////////////////////////////////////////////////////////////

type AuctionRequest struct {
	AuctionID      string
	RecType        string // AUCREQ
	ItemID         string
	AuctionHouseID string // ID of the Auction House managing the auction
	RequestDate    string // Date on which Auction Request was filed
	Status         string // INIT, OPEN, CLOSED (To be Updated by Trgger Auction)
	OpenDate       string // Date on which auction will occur (To be Updated by Trigger Auction)
	CloseDate      string // Date and time when Auction will close (To be Updated by Trigger Auction)
}

/////////////////////////////////////////////////////////////
// POST the transaction after the Auction Completes
// Post an Auction Transaction
// Post an Updated Item Object
// Once an auction request is opened for auctions, a timer is kicked
// off and bids are accepted. When the timer expires, the highest bid
// is selected and converted into a Transaction
// This transaction is a simple view
/////////////////////////////////////////////////////////////

type ItemTransaction struct {
	AuctionID   string
	RecType     string // POSTTRAN
	ItemID      string
	TransType   string // Sale, Buy, Commission
	UserId      string // Buyer or Seller ID
	TransDate   string // Date of Settlement (Buyer or Seller)
	HammerTime  string // Time of hammer strike - SOLD
	HammerPrice string // Total Settlement price
	Details     string // Details about the Transaction
}

////////////////////////////////////////////////////////////////
//  This is a Bid. Bids are accepted only if an auction is OPEN
////////////////////////////////////////////////////////////////

type Bid struct {
	AuctionID string
	RecType   string // BID
	BidNo     string
	ItemID    string
	BuyerID   string // ID Of Buyer - to be verified against the Item CurrentOwnerId
	BidPrice  string // BidPrice > Previous Bid
	BidTime   string // Time the bid was received
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// A Map that holds TableNames and the number of Keys
// This information is used to dynamically Create, Update
// Replace , and Query the Ledger
// In this model all attributes in a table are strings
// The chain code does both validation
// A dummy key like 2016 in some cases is used for a query to get all rows
//
//              "UserTable":        1, Key: UserID
//              "ItemTable":        1, Key: ItemID
//              "UserCatTable":     3, Key: "2016", UserType, UserID
//              "ItemCatTable":     3, Key: "2016", ItemSubject, ItemID
//              "AuctionTable":     1, Key: AuctionID
//              "AucInitTable":     2, Key: Year, AuctionID
//              "AucOpenTable":     2, Key: Year, AuctionID
//              "TransTable":       2, Key: AuctionID, ItemID
//              "BidTable":         2, Key: AuctionID, BidNo
//              "ItemHistoryTable": 4, Key: ItemID, Status, AuctionHouseID(if applicable),date-time
//
/////////////////////////////////////////////////////////////////////////////////////////////////////

func GetNumberOfKeys(tname string) int {
	TableMap := map[string]int{
		"UserTable":        1,
		"ItemTable":        1,
		"UserCatTable":     3,
		"ItemCatTable":     3,
		"AuctionTable":     1,
		"AucInitTable":     2,
		"AucOpenTable":     2,
		"TransTable":       2,
		"BidTable":         2,
		"ItemHistoryTable": 4,
	}
	return TableMap[tname]
}

//////////////////////////////////////////////////////////////
// Invoke Functions based on Function name
// The function name gets resolved to one of the following calls
// during an invoke
//
//////////////////////////////////////////////////////////////
func InvokeFunction(fname string) func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	InvokeFunc := map[string]func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error){
		"PostItem":           PostItem,
		"PostUser":           PostUser,
		"PostAuctionRequest": PostAuctionRequest,
		//"PostBid":            PostBid,
		"OpenAuctionForBids": OpenAuctionForBids,
		"BuyItNow":           BuyItNow,
		//"CloseAuction":       CloseAuction,
		//"CloseOpenAuctions":  CloseOpenAuctions,
	}
	return InvokeFunc[fname]
}

//////////////////////////////////////////////////////////////
// Query Functions based on Function name
//
//////////////////////////////////////////////////////////////
func QueryFunction(fname string) func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	QueryFunc := map[string]func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error){
		"GetItem":             GetItem,
		"GetUser":             GetUser,
		"GetAuctionRequest":   GetAuctionRequest,
		"GetBid":              GetBid,
		"GetLastBid":          GetLastBid,
		"GetHighestBid":       GetHighestBid,
		"GetNoOfBidsReceived": GetNoOfBidsReceived,
		// "GetListOfBids":         GetListOfBids,
		"GetUserListByCat": GetUserListByCat,
		// "GetListOfInitAucs":     GetListOfInitAucs,
		// "GetListOfOpenAucs":     GetListOfOpenAucs,
		// "ValidateItemOwnership": ValidateItemOwnership,
		// "IsItemOnAuction": IsItemOnAuction,
		"GetVersion": GetVersion,
	}
	return QueryFunc[fname]
}

//var myLogger = logging.MustGetLogger("auction_trading")

type SimpleChaincode struct {
}

var gopath string
var ccPath string

////////////////////////////////////////////////////////////////////////////////
// Chain Code Kick-off Main function
////////////////////////////////////////////////////////////////////////////////
func main() {

	// maximize CPU usage for maximum performance
	runtime.GOMAXPROCS(runtime.NumCPU())
	fmt.Println("Starting Item Auction Application chaincode BlueMix ver 0.25 Dated 2016-07-17 15.20.00 ")

	gopath = os.Getenv("GOPATH")
	if len(os.Args) == 2 && strings.EqualFold(os.Args[1], "DEV") {
		fmt.Println("----------------- STARTED IN DEV MODE -------------------- ")
		//set chaincode path for DEV MODE
		ccPath = fmt.Sprintf("%s/src/github.com/hyperledger/fabric/auction/art/artchaincode/", gopath)
	} else {
		fmt.Println("----------------- STARTED IN NET MODE -------------------- ")
		//set chaincode path for NET MODE
		ccPath = fmt.Sprintf("%s/src/github.com/ITPeople-Blockchain/auction/art/artchaincode/", gopath)
	}

	// Start the shim -- running the fabric
	/*err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Println("Error starting Item Fun Application chaincode: %s", err)
	}*/

}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// SimpleChaincode - Init Chaincode implementation - The following sequence of transactions can be used to test the Chaincode
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	// TODO - Include all initialization to be complete before Invoke and Query
	// Uses aucTables to delete tables if they exist and re-create them

	//myLogger.Info("[Trade and Auction Application] Init")
	fmt.Println("[Trade and Auction Application] Init")
	var err error

	for _, val := range aucTables {
		err = stub.DeleteTable(val)
		if err != nil {
			return nil, fmt.Errorf("Init(): DeleteTable of %s  Failed ", val)
		}
		err = InitLedger(stub, val)
		if err != nil {
			return nil, fmt.Errorf("Init(): InitLedger of %s  Failed ", val)
		}
	}
	// Update the ledger with the Application version
	err = stub.PutState("version", []byte(strconv.Itoa(23)))
	if err != nil {
		return nil, err
	}

	fmt.Println("Init() Initialization Complete  : ", args)
	return []byte("Init(): Initialization Complete"), nil
}

////////////////////////////////////////////////////////////////
// SimpleChaincode - INVOKE Chaincode implementation
// User Can Invoke
// - Register a user using PostUser
// - Register an item using PostItem
// - The Owner of the item (User) can request that the item be put on auction using PostAuctionRequest
// - The Auction House can request that the auction request be Opened for bids using OpenAuctionForBids
// - One the auction is OPEN, registered buyers (Buyers) can send in bids vis PostBid
// - No bid is accepted when the status of the auction request is INIT or CLOSED
// - Either manually or by OpenAuctionRequest, the auction can be closed using CloseAuction
// - The CloseAuction creates a transaction and invokes PostTransaction
////////////////////////////////////////////////////////////////

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var err error
	var buff []byte

	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Check Type of Transaction and apply business rules
	// before adding record to the block chain
	// In this version, the assumption is that args[1] specifies recType for all defined structs
	// Newer structs - the recType can be positioned anywhere and ChkReqType will check for recType
	// example:
	// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostBid", "Args":["1111", "BID", "1", "1000", "300", "1200"]}'
	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	if ChkReqType(args) == true {

		InvokeRequest := InvokeFunction(function)
		if InvokeRequest != nil {
			buff, err = InvokeRequest(stub, function, args)
		}
	} else {
		fmt.Println("Invoke() Invalid recType : ", args, "\n")
		return nil, errors.New("Invoke() : Invalid recType : " + args[0])
	}

	return buff, err
}

//////////////////////////////////////////////////////////////////////////////////////////
// SimpleChaincode - QUERY Chaincode implementation
// Client Can Query
// Sample Data
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetUser", "Args": ["4000"]}'
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetItem", "Args": ["2000"]}'
//////////////////////////////////////////////////////////////////////////////////////////

func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var err error
	var buff []byte
	fmt.Println("ID Extracted and Type = ", args[0])
	fmt.Println("Args supplied : ", args)

	if len(args) < 1 {
		fmt.Println("Query() : Include at least 1 arguments Key ")
		return nil, errors.New("Query() : Expecting Transation type and Key value for query")
	}

	QueryRequest := QueryFunction(function)
	if QueryRequest != nil {
		buff, err = QueryRequest(stub, function, args)
	} else {
		fmt.Println("Query() Invalid function call : ", function)
		return nil, errors.New("Query() : Invalid function call : " + function)
	}

	if err != nil {
		fmt.Println("Query() Object not found : ", args[0])
		return nil, errors.New("Query() : Object not found : " + args[0])
	}
	return buff, err
}

//////////////////////////////////////////////////////////////////////////////////////////
// Retrieve Auction applications version Information
// This API is to check whether application has been deployed successfully or not
// example:
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetVersion", "Args": ["version"]}'
//
//////////////////////////////////////////////////////////////////////////////////////////
func GetVersion(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if len(args) < 1 {
		fmt.Println("GetVersion() : Requires 1 argument 'version'")
		return nil, errors.New("GetVersion() : Requires 1 argument 'version'")
	}
	// Get version from the ledger
	version, err := stub.GetState(args[0])
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for version\"}"
		return nil, errors.New(jsonResp)
	}

	if version == nil {
		jsonResp := "{\"Error\":\" auction application version is invalid\"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"version\":\"" + string(version) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)
	return version, nil
}

//////////////////////////////////////////////////////////////////////////////////////////
// Retrieve User Information
// example:
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetUser", "Args": ["100"]}'
//
//////////////////////////////////////////////////////////////////////////////////////////
func GetUser(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Get the Object and Display it
	Avalbytes, err := QueryLedger(stub, "UserTable", args)
	if err != nil {
		fmt.Println("GetUser() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetUser() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetUser() : Response : Successfull -")
	return Avalbytes, nil
}

/////////////////////////////////////////////////////////////////////////////////////////
// Query callback representing the query of a chaincode
// Retrieve a Item by Item ID
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetItem", "Args": ["1000"]}'
/////////////////////////////////////////////////////////////////////////////////////////
func GetItem(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Get the Objects and Display it
	Avalbytes, err := QueryLedger(stub, "ItemTable", args)
	if err != nil {
		fmt.Println("GetItem() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetItem() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetItem() : Response : Successfull ")

	return Avalbytes, nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// Retrieve Auction Information
// This query runs against the AuctionTable
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetAuctionRequest", "Args": ["1111"]}'
// There are two other tables just for query purposes - AucInitTable, AucOpenTable
//
/////////////////////////////////////////////////////////////////////////////////////////////////////
func GetAuctionRequest(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Get the Objects and Display it
	Avalbytes, err := QueryLedger(stub, "AuctionTable", args)
	if err != nil {
		fmt.Println("GetAuctionRequest() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetAuctionRequest() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetAuctionRequest() : Response : Successfull - \n")
	return Avalbytes, nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////
// Retrieve a Bid based on two keys - AucID, BidNo
// A Bid has two Keys - The Auction Request Number and Bid Number
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetLastBid", "Args": ["1111"], "1"}'
//
///////////////////////////////////////////////////////////////////////////////////////////////////
func GetBid(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Check there are 2 Arguments provided as per the the struct - two are computed
	// See example
	if len(args) < 2 {
		fmt.Println("GetBid(): Incorrect number of arguments. Expecting 2 ")
		fmt.Println("GetBid(): ./peer chaincode query -l golang -n mycc -c '{\"Function\": \"GetBid\", \"Args\": [\"1111\",\"6\"]}'")
		return nil, errors.New("GetBid(): Incorrect number of arguments. Expecting 2 ")
	}

	// Get the Objects and Display it
	Avalbytes, err := QueryLedger(stub, "BidTable", args)
	if err != nil {
		fmt.Println("GetBid() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetBid() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetBid() : Response : Successfull -")
	return Avalbytes, nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Create a User Object. The first step is to have users
// registered
// There are different types of users - Traders (TRD), Auction Houses (AH)
// Shippers (SHP), Insurance Companies (INS), Banks (BNK)
// While this version of the chain code does not enforce strict validation
// the business process recomends validating each persona for the service
// they provide or their participation on the auction blockchain, future enhancements will do that
// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostUser", "Args":["100", "USER", "Ashley Hart", "TRD",  "Morrisville Parkway, #216, Morrisville, NC 27560", "9198063535", "ashley@itpeople.com", "SUNTRUST", "00017102345", "0234678"]}'
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func PostUser(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	record, err := CreateUserObject(args[0:]) //
	if err != nil {
		return nil, err
	}
	buff, err := UsertoJSON(record) //

	if err != nil {
		fmt.Println("PostuserObject() : Failed Cannot create object buffer for write : ", args[1])
		return nil, errors.New("PostUser(): Failed Cannot create object buffer for write : " + args[1])
	} else {
		// Update the ledger with the Buffer Data
		// err = stub.PutState(args[0], buff)
		keys := []string{args[0]}
		err = UpdateLedger(stub, "UserTable", keys, buff)
		if err != nil {
			fmt.Println("PostUser() : write error while inserting record")
			return nil, err
		}

		// Post Entry into UserCatTable - i.e. User Category Table
		keys = []string{"2016", args[3], args[0]}
		err = UpdateLedger(stub, "UserCatTable", keys, buff)
		if err != nil {
			fmt.Println("PostUser() : write error while inserting recordinto UserCatTable \n")
			return nil, err
		}
	}

	return buff, err
}

func CreateUserObject(args []string) (UserObject, error) {

	var err error
	var aUser UserObject

	// Check there are 10 Arguments
	if len(args) != 10 {
		fmt.Println("CreateUserObject(): Incorrect number of arguments. Expecting 10 ")
		return aUser, errors.New("CreateUserObject() : Incorrect number of arguments. Expecting 10 ")
	}

	// Validate UserID is an integer

	_, err = strconv.Atoi(args[0])
	if err != nil {
		return aUser, errors.New("CreateUserObject() : User ID should be an integer")
	}

	aUser = UserObject{args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9]}
	fmt.Println("CreateUserObject() : User Object : ", aUser)

	return aUser, nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Create a master Object of the Item
// Since the Owner Changes hands, a record has to be written for each
// Transaction with the updated Encryption Key of the new owner
// Example
//./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostItem", "Args":["1000", "ARTINV", "Shadows by Asppen", "Asppen Messer", "20140202", "Original", "Landscape" , "Canvas", "15 x 15 in", "sample_7.png","$600", "100"]}'
/////////////////////////////////////////////////////////////////////////////////////////////////////////////

func PostItem(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	itemObject, err := CreateItemObject(args[0:])
	if err != nil {
		fmt.Println("PostItem(): Cannot create item object \n")
		return nil, err
	}

	// Convert Item Object to JSON
	buff, err := ARtoJSON(itemObject) //
	if err != nil {
		fmt.Println("PostItem() : Failed Cannot create object buffer for write : ", args[0])
		return nil, errors.New("PostItem(): Failed Cannot create object buffer for write : " + args[0])
	} else {
		// Update the ledger with the Buffer Data
		// err = stub.PutState(args[0], buff)
		keys := []string{args[0]}
		err = UpdateLedger(stub, "ItemTable", keys, buff)
		if err != nil {
			fmt.Println("PostItem() : write error while inserting record\n")
			return buff, err
		}
	}

	return buff, nil
}

func CreateItemObject(args []string) (ItemObject, error) {

	var myItem ItemObject

	// Check there are 12 Arguments provided as per the the struct - two are computed
	if len(args) != 6 {
		fmt.Println("CreateItemObject(): Incorrect number of arguments. Expecting 6 ")
		return myItem, errors.New("CreateItemObject(): Incorrect number of arguments. Expecting 6 ")
	}

	// Append the AES Key, The Encrypted Image Byte Array and the file type
	myItem = ItemObject{args[0], args[1], args[2], args[3], args[4], args[5]}

	fmt.Println("CreateItemObject(): Item Object created: ID# ", myItem.ItemID)

	// Code to Validate the Item Object)
	// If User presents Crypto Key then key is used to validate the picture that is stored as part of the title
	// TODO

	return myItem, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Create an Auction Request
// The owner of an Item, when ready to put the item on an auction
// will create an auction request  and specify a  auction house.
//
// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostAuctionRequest", "Args":["1111", "AUCREQ", "1700", "200", "400", "04012016", "1200", "INIT", "2016-05-20 11:00:00.3 +0000 UTC","2016-05-23 11:00:00.3 +0000 UTC"]}'
//
// The start and end time of the auction are actually assigned when the auction is opened  by OpenAuctionForBids()
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func PostAuctionRequest(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	ar, err := CreateAuctionRequest(args[0:])
	if err != nil {
		return nil, err
	}

	// Validate Auction House to check it is a registered User
	aucHouse, err := ValidateMember(stub, ar.AuctionHouseID)
	fmt.Println("Auction House information  ", aucHouse, " ID: ", ar.AuctionHouseID)
	if err != nil {
		fmt.Println("PostAuctionRequest() : Failed Auction House not Registered in Blockchain ", ar.AuctionHouseID)
		return nil, err
	}

	// Validate Item record
	itemObject, err := ValidateItemSubmission(stub, ar.ItemID)
	if err != nil {
		fmt.Println("PostAuctionRequest() : Failed Could not Validate Item Object in Blockchain ", ar.ItemID)
		return itemObject, err
	}

	// Convert AuctionRequest to JSON
	buff, err := AucReqtoJSON(ar) // Converting the auction request struct to []byte array
	if err != nil {
		fmt.Println("PostAuctionRequest() : Failed Cannot create object buffer for write : ", args[1])
		return nil, errors.New("PostAuctionRequest(): Failed Cannot create object buffer for write : " + args[1])
	} else {
		// Update the ledger with the Buffer Data
		//err = stub.PutState(args[0], buff)
		keys := []string{args[0]}
		err = UpdateLedger(stub, "AuctionTable", keys, buff)
		if err != nil {
			fmt.Println("PostAuctionRequest() : write error while inserting record\n")
			return buff, err
		}

		//An entry is made in the AuctionInitTable that this Item has been placed for Auction
		// The UI can pull all items available for auction and the item can be Opened for accepting bids
		// The 2016 is a dummy key and has notr value other than to get all rows

		keys = []string{"2016", args[0]}
		err = UpdateLedger(stub, "AucInitTable", keys, buff)
		if err != nil {
			fmt.Println("PostAuctionRequest() : write error while inserting record into AucInitTable \n")
			return buff, err
		}

	}

	return buff, err
}

func CreateAuctionRequest(args []string) (AuctionRequest, error) {
	var aucReg AuctionRequest

	// Check there are 11 Arguments
	// See example -- The Open and Close Dates are Dummy, and will be set by open auction
	// '{"Function": "PostAuctionRequest", "Args":["1111", "AUCREQ", "1000", "200", "100", "04012016", "1200", "1800",
	//   "INIT", "2016-05-20 11:00:00.3 +0000 UTC","2016-05-23 11:00:00.3 +0000 UTC"]}'
	if len(args) != 8 {
		fmt.Println("CreateAuctionRegistrationObject(): Incorrect number of arguments. Expecting 8 ")
		return aucReg, errors.New("CreateAuctionRegistrationObject() : Incorrect number of arguments. Expecting 11 ")
	}

	// Validate UserID is an integer . I think this redundant and can be avoided

	/*err = validateID(args[0])
	if err != nil {
		return aucReg, errors.New("CreateAuctionRequest() : User ID should be an integer")
	}*/

	aucReg = AuctionRequest{args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7]}
	fmt.Println("CreateAuctionObject() : Auction Registration : ", aucReg)

	return aucReg, nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Create a Bid Object
// Once an Item has been opened for auction, bids can be submitted as long as the auction is "OPEN"
//./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostBid", "Args":["1111", "BID", "1", "1000", "300", "1200"]}'
//./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostBid", "Args":["1111", "BID", "2", "1000", "400", "3000"]}'
//
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/*func PostBid(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	bid, err := CreateBidObject(args[0:]) //
	if err != nil {
		return nil, err
	}

	// Reject the Bid if the Buyer Information Is not Valid or not registered on the Block Chain
	buyerInfo, err := ValidateMember(stub, args[4])
	fmt.Println("Buyer information  ", buyerInfo, "  ", args[4])
	if err != nil {
		fmt.Println("PostBid() : Failed Buyer not registered on the block-chain ", args[4])
		return nil, err
	}

	///////////////////////////////////////
	// Reject Bid if Auction is not "OPEN"
	///////////////////////////////////////
	RBytes, err := GetAuctionRequest(stub, "GetAuctionRequest", []string{args[0]})
	if err != nil {
		fmt.Println("PostBid() : Cannot find Auction record ", args[0])
		return nil, errors.New("PostBid(): Cannot find Auction record : " + args[0])
	}

	aucR, err := JSONtoAucReq(RBytes)
	if err != nil {
		fmt.Println("PostBid() : Cannot UnMarshall Auction record")
		return nil, errors.New("PostBid(): Cannot UnMarshall Auction record: " + args[0])
	}

	if aucR.Status != "OPEN" {
		fmt.Println("PostBid() : Cannot accept Bid as Auction is not OPEN ", args[0])
		return nil, errors.New("PostBid(): Cannot accept Bid as Auction is not OPEN : " + args[0])
	}

	///////////////////////////////////////////////////////////////////
	// Reject Bid if the time bid was received is > Auction Close Time
	///////////////////////////////////////////////////////////////////
	if tCompare(bid.BidTime, aucR.CloseDate) == false {
		fmt.Println("PostBid() Failed : BidTime past the Auction Close Time")
		return nil, fmt.Errorf("PostBid() Failed : BidTime past the Auction Close Time %s, %s", bid.BidTime, aucR.CloseDate)
	}

	//////////////////////////////////////////////////////////////////
	// Reject Bid if Item ID on Bid does not match Item ID on Auction
	//////////////////////////////////////////////////////////////////
	if aucR.ItemID != bid.ItemID {
		fmt.Println("PostBid() Failed : Item ID mismatch on bid. Bid Rejected")
		return nil, errors.New("PostBid() : Item ID mismatch on Bid. Bid Rejected")
	}

	//////////////////////////////////////////////////////////////////////
	// Reject Bid if Bid Price is less than Reserve Price
	// Convert Bid Price and Reserve Price to Integer (TODO - Float)
	//////////////////////////////////////////////////////////////////////
	bp, err := strconv.Atoi(bid.BidPrice)
	if err != nil {
		fmt.Println("PostBid() Failed : Bid price should be an integer")
		return nil, errors.New("PostBid() : Bid price should be an integer")
	}

	hp, err := strconv.Atoi(aucR.ReservePrice)
	if err != nil {
		return nil, errors.New("PostItem() : Reserve Price should be an integer")
	}

	// Check if Bid Price is > Auction Request Reserve Price
	if bp < hp {
		return nil, errors.New("PostItem() : Bid Price must be greater than Reserve Price")
	}

	////////////////////////////
	// Post or Accept the Bid
	////////////////////////////
	buff, err := BidtoJSON(bid) //

	if err != nil {
		fmt.Println("PostBid() : Failed Cannot create object buffer for write : ", args[1])
		return nil, errors.New("PostBid(): Failed Cannot create object buffer for write : " + args[1])
	} else {
		// Update the ledger with the Buffer Data
		// err = stub.PutState(args[0], buff)
		keys := []string{args[0], args[2]}
		err = UpdateLedger(stub, "BidTable", keys, buff)
		if err != nil {
			fmt.Println("PostBidTable() : write error while inserting record\n")
			return buff, err
		}
	}

	return buff, err
}

func CreateBidObject(args []string) (Bid, error) {
	var err error
	var aBid Bid

	// Check there are 11 Arguments
	// See example
	if len(args) != 6 {
		fmt.Println("CreateBidObject(): Incorrect number of arguments. Expecting 6 ")
		return aBid, errors.New("CreateBidObject() : Incorrect number of arguments. Expecting 6 ")
	}

	// Validate Bid is an integer

	_, err = strconv.Atoi(args[0])
	if err != nil {
		return aBid, errors.New("CreateBidObject() : Bid ID should be an integer")
	}

	_, err = strconv.Atoi(args[2])
	if err != nil {
		return aBid, errors.New("CreateBidObject() : Bid ID should be an integer")
	}

	bidTime := time.Now().Format("2006-01-02 15:04:05")

	aBid = Bid{args[0], args[1], args[2], args[3], args[4], args[5], bidTime}
	fmt.Println("CreateBidObject() : Bid Object : ", aBid)

	return aBid, nil
}*/

//////////////////////////////////////////////////////////
// JSON To args[] - return a map of the JSON string
//////////////////////////////////////////////////////////
func JSONtoArgs(Avalbytes []byte) (map[string]interface{}, error) {

	var data map[string]interface{}

	if err := json.Unmarshal(Avalbytes, &data); err != nil {
		return nil, err
	}

	return data, nil
}

//////////////////////////////////////////////////////////
// Variation of the above - return value from a JSON string
//////////////////////////////////////////////////////////

func GetKeyValue(Avalbytes []byte, key string) string {
	var dat map[string]interface{}
	if err := json.Unmarshal(Avalbytes, &dat); err != nil {
		panic(err)
	}

	val := dat[key].(string)
	return val
}

//////////////////////////////////////////////////////////
// Time and Date Comparison
// tCompare("2016-06-28 18:40:57", "2016-06-27 18:45:39")
//////////////////////////////////////////////////////////
func tCompare(t1 string, t2 string) bool {

	layout := "2006-01-02 15:04:05"
	bidTime, err := time.Parse(layout, t1)
	if err != nil {
		fmt.Println("tCompare() Failed : time Conversion error on t1")
		return false
	}

	aucCloseTime, err := time.Parse(layout, t2)
	if err != nil {
		fmt.Println("tCompare() Failed : time Conversion error on t2")
		return false
	}

	if bidTime.Before(aucCloseTime) {
		return true
	}

	return false
}

//////////////////////////////////////////////////////////
// Converts JSON String to an ART Object
//////////////////////////////////////////////////////////
func JSONtoAR(data []byte) (ItemObject, error) {

	ar := ItemObject{}
	err := json.Unmarshal([]byte(data), &ar)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	return ar, err
}

//////////////////////////////////////////////////////////
// Converts an ART Object to a JSON String
//////////////////////////////////////////////////////////
func ARtoJSON(ar ItemObject) ([]byte, error) {

	ajson, err := json.Marshal(ar)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

//////////////////////////////////////////////////////////
// Converts an BID to a JSON String
//////////////////////////////////////////////////////////
func ItemLogtoJSON(item ItemLog) ([]byte, error) {

	ajson, err := json.Marshal(item)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func JSONtoItemLog(ithis []byte) (ItemLog, error) {

	item := ItemLog{}
	err := json.Unmarshal(ithis, &item)
	if err != nil {
		fmt.Println("JSONtoItemLog error: ", err)
		return item, err
	}
	return item, err
}

//////////////////////////////////////////////////////////
// Converts an Auction Request to a JSON String
//////////////////////////////////////////////////////////
func AucReqtoJSON(ar AuctionRequest) ([]byte, error) {

	ajson, err := json.Marshal(ar)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func JSONtoAucReq(areq []byte) (AuctionRequest, error) {

	ar := AuctionRequest{}
	err := json.Unmarshal(areq, &ar)
	if err != nil {
		fmt.Println("JSONtoAucReq error: ", err)
		return ar, err
	}
	return ar, err
}

//////////////////////////////////////////////////////////
// Converts BID Object to JSON String
//////////////////////////////////////////////////////////
func BidtoJSON(myHand Bid) ([]byte, error) {

	ajson, err := json.Marshal(myHand)
	if err != nil {
		fmt.Println("BidtoJSON error: ", err)
		return nil, err
	}
	return ajson, nil
}

//////////////////////////////////////////////////////////
// Converts JSON String to BID Object
//////////////////////////////////////////////////////////
func JSONtoBid(areq []byte) (Bid, error) {

	myHand := Bid{}
	err := json.Unmarshal(areq, &myHand)
	if err != nil {
		fmt.Println("JSONtoBid error: ", err)
		return myHand, err
	}
	return myHand, err
}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func UsertoJSON(user UserObject) ([]byte, error) {

	ajson, err := json.Marshal(user)
	if err != nil {
		fmt.Println("UsertoJSON error: ", err)
		return nil, err
	}
	fmt.Println("UsertoJSON created: ", ajson)
	return ajson, nil
}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func JSONtoUser(user []byte) (UserObject, error) {

	ur := UserObject{}
	err := json.Unmarshal(user, &ur)
	if err != nil {
		fmt.Println("JSONtoUser error: ", err)
		return ur, err
	}
	fmt.Println("JSONtoUser created: ", ur)
	return ur, err
}

//////////////////////////////////////////////
// Validates an ID for Well Formed
//////////////////////////////////////////////

func validateID(id string) error {
	// Validate UserID is an integer

	_, err := strconv.Atoi(id)
	if err != nil {
		return errors.New("validateID(): User ID should be an integer")
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////
// Validate if the User Information Exists
// in the block-chain
////////////////////////////////////////////////////////////////////////////
func ValidateMember(stub shim.ChaincodeStubInterface, owner string) ([]byte, error) {

	// Get the Item Objects and Display it
	// Avalbytes, err := stub.GetState(owner)
	args := []string{owner, "USER"}
	Avalbytes, err := QueryLedger(stub, "UserTable", args)

	if err != nil {
		fmt.Println("ValidateMember() : Failed - Cannot find valid owner record for ART  ", owner)
		jsonResp := "{\"Error\":\"Failed to get Owner Object Data for " + owner + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("ValidateMember() : Failed - Incomplete owner record for ART  ", owner)
		jsonResp := "{\"Error\":\"Failed - Incomplete information about the owner for " + owner + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("ValidateMember() : Validated Item Owner:\n", owner)
	return Avalbytes, nil
}

////////////////////////////////////////////////////////////////////////////
// Validate if the User Information Exists
// in the block-chain
////////////////////////////////////////////////////////////////////////////
func ValidateItemSubmission(stub shim.ChaincodeStubInterface, artId string) ([]byte, error) {

	// Get the Item Objects and Display it
	args := []string{artId, "ARTINV"}
	Avalbytes, err := QueryLedger(stub, "ItemTable", args)
	if err != nil {
		fmt.Println("ValidateItemSubmission() : Failed - Cannot find valid owner record for ART  ", artId)
		jsonResp := "{\"Error\":\"Failed to get Owner Object Data for " + artId + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("ValidateItemSubmission() : Failed - Incomplete owner record for ART  ", artId)
		jsonResp := "{\"Error\":\"Failed - Incomplete information about the owner for " + artId + "\"}"
		return nil, errors.New(jsonResp)
	}

	//fmt.Println("ValidateItemSubmission() : Validated Item Owner:", Avalbytes)
	return Avalbytes, nil
}

////////////////////////////////////////////////////////////////////////////
// Open a Ledgers if one does not exist
// These ledgers will be used to write /  read data
// Use names are listed in aucTables {}
// THIS FUNCTION REPLACES ALL THE INIT Functions below
//  - InitUserReg()
//  - InitAucReg()
//  - InitBidReg()
//  - InitItemReg()
//  - InitItemMaster()
//  - InitTransReg()
//  - InitAuctionTriggerReg()
//  - etc. etc.
////////////////////////////////////////////////////////////////////////////
func InitLedger(stub shim.ChaincodeStubInterface, tableName string) error {

	// Generic Table Creation Function - requires Table Name and Table Key Entry
	// Create Table - Get number of Keys the tables supports
	// This version assumes all Keys are String and the Data is Bytes
	// This Function can replace all other InitLedger function in this app such as InitItemLedger()

	nKeys := GetNumberOfKeys(tableName)
	if nKeys < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
		fmt.Println("Auction_Application: Failed creating Table ", tableName)
		return errors.New("Auction_Application: Failed creating Table " + tableName)
	}

	var columnDefsForTbl []*shim.ColumnDefinition

	for i := 0; i < nKeys; i++ {
		columnDef := shim.ColumnDefinition{Name: "keyName" + strconv.Itoa(i), Type: shim.ColumnDefinition_STRING, Key: true}
		columnDefsForTbl = append(columnDefsForTbl, &columnDef)
	}

	columnLastTblDef := shim.ColumnDefinition{Name: "Details", Type: shim.ColumnDefinition_BYTES, Key: false}
	columnDefsForTbl = append(columnDefsForTbl, &columnLastTblDef)

	// Create the Table (Nil is returned if the Table exists or if the table is created successfully
	err := stub.CreateTable(tableName, columnDefsForTbl)

	if err != nil {
		fmt.Println("Auction_Application: Failed creating Table ", tableName)
		return errors.New("Auction_Application: Failed creating Table " + tableName)
	}

	return err
}

////////////////////////////////////////////////////////////////////////////
// Open a User Registration Table if one does not exist
// Register users into this table
////////////////////////////////////////////////////////////////////////////
func UpdateLedger(stub shim.ChaincodeStubInterface, tableName string, keys []string, args []byte) error {

	nKeys := GetNumberOfKeys(tableName)
	if nKeys < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
	}

	var columns []*shim.Column

	for i := 0; i < nKeys; i++ {
		col := shim.Column{Value: &shim.Column_String_{String_: keys[i]}}
		columns = append(columns, &col)
	}

	lastCol := shim.Column{Value: &shim.Column_Bytes{Bytes: []byte(args)}}
	columns = append(columns, &lastCol)

	row := shim.Row{columns}
	ok, err := stub.InsertRow(tableName, row)
	if err != nil {
		return fmt.Errorf("UpdateLedger: InsertRow into "+tableName+" Table operation failed. %s", err)
	}
	if !ok {
		return errors.New("UpdateLedger: InsertRow into " + tableName + " Table failed. Row with given key " + keys[0] + " already exists")
	}

	fmt.Println("UpdateLedger: InsertRow into ", tableName, " Table operation Successful. ")
	return nil
}

////////////////////////////////////////////////////////////////////////////
// Open a User Registration Table if one does not exist
// Register users into this table
////////////////////////////////////////////////////////////////////////////
func DeleteFromLedger(stub shim.ChaincodeStubInterface, tableName string, keys []string) error {
	var columns []shim.Column

	//nKeys := GetNumberOfKeys(tableName)
	nCol := len(keys)
	if nCol < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
		return errors.New("DeleteFromLedger failed. Must include at least key values")
	}

	for i := 0; i < nCol; i++ {
		colNext := shim.Column{Value: &shim.Column_String_{String_: keys[i]}}
		columns = append(columns, colNext)
	}

	err := stub.DeleteRow(tableName, columns)
	if err != nil {
		return fmt.Errorf("DeleteFromLedger operation failed. %s", err)
	}

	fmt.Println("DeleteFromLedger: DeleteRow from ", tableName, " Table operation Successful. ")
	return nil
}

////////////////////////////////////////////////////////////////////////////
// Replaces the Entry in the Ledger
//
////////////////////////////////////////////////////////////////////////////
func ReplaceLedgerEntry(stub shim.ChaincodeStubInterface, tableName string, keys []string, args []byte) error {

	nKeys := GetNumberOfKeys(tableName)
	if nKeys < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
	}

	var columns []*shim.Column

	for i := 0; i < nKeys; i++ {
		col := shim.Column{Value: &shim.Column_String_{String_: keys[i]}}
		columns = append(columns, &col)
	}

	lastCol := shim.Column{Value: &shim.Column_Bytes{Bytes: []byte(args)}}
	columns = append(columns, &lastCol)

	row := shim.Row{columns}
	ok, err := stub.ReplaceRow(tableName, row)
	if err != nil {
		return fmt.Errorf("ReplaceLedgerEntry: Replace Row into "+tableName+" Table operation failed. %s", err)
	}
	if !ok {
		return errors.New("ReplaceLedgerEntry: Replace Row into " + tableName + " Table failed. Row with given key " + keys[0] + " already exists")
	}

	fmt.Println("ReplaceLedgerEntry: Replace Row in ", tableName, " Table operation Successful. ")
	return nil
}

////////////////////////////////////////////////////////////////////////////
// Query a User Object by Table Name and Key
////////////////////////////////////////////////////////////////////////////
func QueryLedger(stub shim.ChaincodeStubInterface, tableName string, args []string) ([]byte, error) {

	var columns []shim.Column
	nCol := GetNumberOfKeys(tableName)
	for i := 0; i < nCol; i++ {
		colNext := shim.Column{Value: &shim.Column_String_{String_: args[i]}}
		columns = append(columns, colNext)
	}

	row, err := stub.GetRow(tableName, columns)
	fmt.Println("Length or number of rows retrieved ", len(row.Columns))

	if len(row.Columns) == 0 {
		jsonResp := "{\"Error\":\"Failed retrieving data " + args[0] + ". \"}"
		fmt.Println("Error retrieving data record for Key = ", args[0], "Error : ", jsonResp)
		return nil, errors.New(jsonResp)
	}

	//fmt.Println("User Query Response:", row)
	//jsonResp := "{\"Owner\":\"" + string(row.Columns[nCol].GetBytes()) + "\"}"
	//fmt.Println("User Query Response:%s\n", jsonResp)
	Avalbytes := row.Columns[nCol].GetBytes()

	// Perform Any additional processing of data
	fmt.Println("QueryLedger() : Successful - Proceeding to ProcessRequestType ")
	err = ProcessQueryResult(stub, Avalbytes, args)
	if err != nil {
		fmt.Println("QueryLedger() : Cannot create object  : ", args[1])
		jsonResp := "{\"QueryLedger() Error\":\" Cannot create Object for key " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}
	return Avalbytes, nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// Get List of Bids for an Auction
// in the block-chain --
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetListOfBids", "Args": ["1111"]}'
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetLastBid", "Args": ["1111"]}'
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetHighestBid", "Args": ["1111"]}'
/////////////////////////////////////////////////////////////////////////////////////////////////////
/*func GetListOfBids(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	rows, err := GetList(stub, "BidTable", args)
	if err != nil {
		return nil, fmt.Errorf("GetListOfBids operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("BidTable")

	tlist := make([]Bid, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		bid, err := JSONtoBid(ts)
		if err != nil {
			fmt.Println("GetListOfBids() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetListOfBids() operation failed. %s", err)
		}
		tlist[i] = bid
	}

	jsonRows, _ := json.Marshal(tlist)

	fmt.Println("List of Bids Requested : ", jsonRows)
	return jsonRows, nil

}
*/
////////////////////////////////////////////////////////////////////////////////////////////////////////
// Get List of Auctions that have been initiated
// in the block-chain
// This is a fixed Query to be issued as below
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetListOfInitAucs", "Args": ["2016"]}'
////////////////////////////////////////////////////////////////////////////////////////////////////////
/*func GetListOfInitAucs(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	rows, err := GetList(stub, "AucInitTable", args)
	if err != nil {
		return nil, fmt.Errorf("GetListOfInitAucs operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("AucInitTable")

	tlist := make([]AuctionRequest, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoAucReq(ts)
		if err != nil {
			fmt.Println("GetListOfInitAucs() Failed : Ummarshall error")
			return nil, fmt.Errorf("getBillForMonth() operation failed. %s", err)
		}
		tlist[i] = ar
	}

	jsonRows, _ := json.Marshal(tlist)

	//fmt.Println("List of Auctions Requested : ", jsonRows)
	return jsonRows, nil

}*/

////////////////////////////////////////////////////////////////////////////
// Get List of Open Auctions  for which bids can be supplied
// in the block-chain
// This is a fixed Query to be issued as below
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetListOfOpenAucs", "Args": ["2016"]}'
////////////////////////////////////////////////////////////////////////////
/*func GetListOfOpenAucs(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	rows, err := GetList(stub, "AucOpenTable", args)
	if err != nil {
		return nil, fmt.Errorf("GetListOfOpenAucs operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("AucOpenTable")

	tlist := make([]AuctionRequest, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoAucReq(ts)
		if err != nil {
			fmt.Println("GetListOfOpenAucs() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetListOfOpenAucs() operation failed. %s", err)
		}
		tlist[i] = ar
	}

	jsonRows, _ := json.Marshal(tlist)

	//fmt.Println("List of Open Auctions : ", jsonRows)
	return jsonRows, nil

}*/

////////////////////////////////////////////////////////////////////////////
// Get a List of Users by Category
// in the block-chain
////////////////////////////////////////////////////////////////////////////
func GetUserListByCat(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	// Check there are 1 Arguments provided as per the the struct - two are computed
	// See example
	if len(args) < 1 {
		fmt.Println("GetUserListByCat(): Incorrect number of arguments. Expecting 1 ")
		fmt.Println("GetUserListByCat(): ./peer chaincode query -l golang -n mycc -c '{\"Function\": \"GetUserListByCat\", \"Args\": [\"AH\"]}'")
		return nil, errors.New("CreateUserObject(): Incorrect number of arguments. Expecting 1 ")
	}

	rows, err := GetList(stub, "UserCatTable", args)
	if err != nil {
		return nil, fmt.Errorf("GetUserListByCat() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("UserCatTable")

	tlist := make([]UserObject, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		uo, err := JSONtoUser(ts)
		if err != nil {
			fmt.Println("GetUserListByCat() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetUserListByCat() operation failed. %s", err)
		}
		tlist[i] = uo
	}

	jsonRows, _ := json.Marshal(tlist)

	//fmt.Println("All Users : ", jsonRows)
	return jsonRows, nil

}

////////////////////////////////////////////////////////////////////////////
// Get a List of Rows based on query criteria from the OBC
//
////////////////////////////////////////////////////////////////////////////
func GetList(stub shim.ChaincodeStubInterface, tableName string, args []string) ([]shim.Row, error) {
	var columns []shim.Column

	nKeys := GetNumberOfKeys(tableName)
	nCol := len(args)
	if nCol < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
		return nil, errors.New("GetList failed. Must include at least key values")
	}

	for i := 0; i < nCol; i++ {
		colNext := shim.Column{Value: &shim.Column_String_{String_: args[i]}}
		columns = append(columns, colNext)
	}

	rowChannel, err := stub.GetRows(tableName, columns)
	if err != nil {
		return nil, fmt.Errorf("GetList operation failed. %s", err)
	}
	var rows []shim.Row
	for {
		select {
		case row, ok := <-rowChannel:
			if !ok {
				rowChannel = nil
			} else {
				rows = append(rows, row)
				//If required enable for debugging
				//fmt.Println(row)
			}
		}
		if rowChannel == nil {
			break
		}
	}

	fmt.Println("Number of Keys retrieved : ", nKeys)
	fmt.Println("Number of rows retrieved : ", len(rows))
	return rows, nil
}

////////////////////////////////////////////////////////////////////////////
// Get The Highest Bid Received so far for an Auction
// in the block-chain
////////////////////////////////////////////////////////////////////////////
func GetLastBid(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	tn := "BidTable"
	rows, err := GetList(stub, tn, args)
	if err != nil {
		return nil, fmt.Errorf("GetLastBid operation failed. %s", err)
	}
	nCol := GetNumberOfKeys(tn)
	var Avalbytes []byte
	var dat map[string]interface{}
	layout := "2006-01-02 15:04:05"
	highestTime, err := time.Parse(layout, layout)

	for i := 0; i < len(rows); i++ {
		currentBid := rows[i].Columns[nCol].GetBytes()
		if err := json.Unmarshal(currentBid, &dat); err != nil {
			fmt.Println("GetHighestBid() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetHighestBid(0 operation failed. %s", err)
		}
		bidTime, err := time.Parse(layout, dat["BidTime"].(string))
		if err != nil {
			fmt.Println("GetLastBid() Failed : time Conversion error on BidTime")
			return nil, fmt.Errorf("GetHighestBid() Int Conversion error on BidPrice! failed. %s", err)
		}

		if bidTime.Sub(highestTime) > 0 {
			highestTime = bidTime
			Avalbytes = currentBid
		}
	}

	return Avalbytes, nil

}

////////////////////////////////////////////////////////////////////////////
// Get The Highest Bid Received so far for an Auction
// in the block-chain
////////////////////////////////////////////////////////////////////////////
func GetNoOfBidsReceived(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	tn := "BidTable"
	rows, err := GetList(stub, tn, args)
	if err != nil {
		return nil, fmt.Errorf("GetLastBid operation failed. %s", err)
	}
	nBids := len(rows)
	return []byte(strconv.Itoa(nBids)), nil
}

////////////////////////////////////////////////////////////////////////////
// Get the Highest Bid in the List
//
////////////////////////////////////////////////////////////////////////////
func GetHighestBid(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	tn := "BidTable"
	rows, err := GetList(stub, tn, args)
	if err != nil {
		return nil, fmt.Errorf("GetLastBid operation failed. %s", err)
	}
	nCol := GetNumberOfKeys(tn)
	var Avalbytes []byte
	var dat map[string]interface{}
	var bidPrice, highestBid int
	highestBid = 0

	for i := 0; i < len(rows); i++ {
		currentBid := rows[i].Columns[nCol].GetBytes()
		if err := json.Unmarshal(currentBid, &dat); err != nil {
			fmt.Println("GetHighestBid() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetHighestBid(0 operation failed. %s", err)
		}
		bidPrice, err = strconv.Atoi(dat["BidPrice"].(string))
		if err != nil {
			fmt.Println("GetHighestBid() Failed : Int Conversion error on BidPrice")
			return nil, fmt.Errorf("GetHighestBid() Int Conversion error on BidPrice! failed. %s", err)
		}

		if bidPrice >= highestBid {
			highestBid = bidPrice
			Avalbytes = currentBid
		}
	}

	return Avalbytes, nil
}

/////////////////////////////////////////////////////////////////
// This function checks the incoming args stuff for a valid record
// type entry as per the declared array recType[]
// The assumption is that rectType can be anywhere in the args or struct
// not necessarily in args[1] as per my old logic
// The Request type is used to process the record accordingly
/////////////////////////////////////////////////////////////////
func IdentifyReqType(args []string) string {
	for _, rt := range args {
		for _, val := range recType {
			if val == rt {
				return rt
			}
		}
	}
	return "DEFAULT"
}

/////////////////////////////////////////////////////////////////
// This function checks the incoming args stuff for a valid record
// type entry as per the declared array recType[]
// The assumption is that rectType can be anywhere in the args or struct
// not necessarily in args[1] as per my old logic
// The Request type is used to process the record accordingly
/////////////////////////////////////////////////////////////////
func ChkReqType(args []string) bool {
	for _, rt := range args {
		for _, val := range recType {
			if val == rt {
				return true
			}
		}
	}
	return false
}

/////////////////////////////////////////////////////////////////
// Checks if the incoming invoke has a valid requesType
// The Request type is used to process the record accordingly
// Old Logic (see new logic up)
/////////////////////////////////////////////////////////////////
func CheckRequestType(rt string) bool {
	for _, val := range recType {
		if val == rt {
			fmt.Println("CheckRequestType() : Valid Request Type , val : ", val, rt, "\n")
			return true
		}
	}
	fmt.Println("CheckRequestType() : Invalid Request Type , val : ", rt, "\n")
	return false
}

/////////////////////////////////////////////////////////////////////////////////////////////
// Return the right Object Buffer after validation to write to the ledger
// var recType = []string{"ARTINV", "USER", "BID", "AUCREQ", "POSTTRAN", "OPENAUC", "CLAUC"}
/////////////////////////////////////////////////////////////////////////////////////////////

func ProcessQueryResult(stub shim.ChaincodeStubInterface, Avalbytes []byte, args []string) error {

	// Identify Record Type by scanning the args for one of the recTypes
	// This is kind of a post-processor once the query fetches the results
	// RecType is the style of programming in the punch card days ..
	// ... well

	var dat map[string]interface{}

	if err := json.Unmarshal(Avalbytes, &dat); err != nil {
		panic(err)
	}

	var recType string
	recType = dat["RecType"].(string)
	switch recType {

	case "ARTINV":

		ar, err := JSONtoAR(Avalbytes) //
		if err != nil {
			fmt.Println("ProcessRequestType(): Cannot create itemObject \n", ar)
			return err
		}
		return err

	case "USER":
		ur, err := JSONtoUser(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", ur)
		return err

	case "AUCREQ":
	case "OPENAUC":
	case "CLAUC":
		ar, err := JSONtoAucReq(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", ar)
		return err
	case "BID":
		bid, err := JSONtoBid(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", bid)
		return err
	case "DEFAULT":
		return nil
	case "XFER":
		return nil
	case "VERIFY":
		return nil
	default:

		return errors.New("Unknown")
	}
	return nil

}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Trigger the Auction
// Structure of args auctionReqID, RecType, Duration in Minutes ( 3 = 3 minutes)
// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "OpenAuctionForBids", "Args":["1111", "OPENAUC", "3"]}'
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func OpenAuctionForBids(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	// Fetch Auction Object and check its Status
	Avalbytes, err := QueryLedger(stub, "AuctionTable", args)
	if err != nil {
		fmt.Println("OpenAuctionForBids(): Auction Object Retrieval Failed ")
		return nil, errors.New("OpenAuctionForBids(): Auction Object Retrieval Failed ")
	}

	aucR, err := JSONtoAucReq(Avalbytes)
	if err != nil {
		fmt.Println("OpenAuctionForBids(): Auction Object Unmarshalling Failed ")
		return nil, errors.New("OpenAuctionForBids(): Auction Object UnMarshalling Failed ")
	}

	if aucR.Status == "CLOSED" {
		fmt.Println("OpenAuctionForBids(): Auction is Closed - Cannot Open for new bids ")
		return nil, errors.New("OpenAuctionForBids(): is Closed - Cannot Open for new bids Failed ")
	}

	// Calculate Time Now and Duration of Auction

	// Validate arg[1]  is an integer as it represents Duration in Minutes
	aucDuration, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Println("OpenAuctionForBids(): Auction Duration is an integer that represents minute! OpenAuctionForBids() Failed ")
		return nil, errors.New("OpenAuctionForBids(): Auction Duration is an integer that represents minute! OpenAuctionForBids() Failed ")
	}

	aucStartDate := time.Now()
	aucEndDate := aucStartDate.Add(time.Duration(aucDuration) * time.Minute)
	//sleepTime := time.Duration(aucDuration * 60 * 1000 * 1000 * 1000)

	//  Update Auction Object
	aucR.OpenDate = aucStartDate.Format("2006-01-02 15:04:05")
	aucR.CloseDate = aucEndDate.Format("2006-01-02 15:04:05")
	aucR.Status = "OPEN"

	buff, err := UpdateAuctionStatus(stub, "AuctionTable", aucR)
	if err != nil {
		fmt.Println("OpenAuctionForBids(): UpdateAuctionStatus() Failed ")
		return nil, errors.New("OpenAuctionForBids(): UpdateAuctionStatus() Failed ")
	}

	// Remove the Auction from INIT Bucket and move to OPEN bucket
	// This was designed primarily to help the UI

	keys := []string{"2016", aucR.AuctionID}
	err = DeleteFromLedger(stub, "AucInitTable", keys)
	if err != nil {
		fmt.Println("OpenAuctionForBids(): DeleteFromLedger() Failed ")
		return nil, errors.New("OpenAuctionForBids(): DeleteFromLedger() Failed ")
	}

	// Add the Auction to Open Bucket
	err = UpdateLedger(stub, "AucOpenTable", keys, buff)
	if err != nil {
		fmt.Println("OpenAuctionForBids() : write error while inserting record into AucInitTable \n")
		return buff, err
	}

	// Initiate Timer for the duration of the Auction
	// Bids are accepted as long as the timer is alive
	/*go func(aucR AuctionRequest, sleeptime time.Duration) ([]byte, error) {
		fmt.Println("OpenAuctionForBids(): Sleeping for ", sleeptime)
		time.Sleep(sleeptime)

		// Exec The following Command from the shell
		ShellCmdToCloseAuction(aucR.AuctionID)
		return nil, err
	}(aucR, sleepTime)*/
	return buff, err
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Create a Command to execute Close Auction From the Command line
// cloaseauction.sh is created and then executed as seen below
// The file contains just one line
// /opt/gopath/src/github.com/hyperledger/fabric/peer chaincode invoke -l golang -n mycc -c '{"Function": "CloseAuction", "Args": ["1111","AUCREQ"]}'
// This approach has been used as opposed to exec.Command... because additional logic to gather environment variables etc. is required
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func ShellCmdToCloseAuction(aucID string) error {
	gopath := os.Getenv("GOPATH")
	cdir := fmt.Sprintf("cd %s/src/github.com/hyperledger/fabric/", gopath)
	argStr := "'{\"Function\": \"CloseAuction\", \"Args\": [\"" + aucID + "\"," + "\"AUCREQ\"" + "]}'"
	argStr = fmt.Sprintf("%s/src/github.com/hyperledger/fabric/peer/peer chaincode invoke -l golang -n mycc -c %s", gopath, argStr)

	fileHandle, _ := os.Create(fmt.Sprintf("%s/src/github.com/hyperledger/fabric/peer/closeauction.sh", gopath))
	writer := bufio.NewWriter(fileHandle)
	defer fileHandle.Close()

	fmt.Fprintln(writer, cdir)
	fmt.Fprintln(writer, argStr)
	writer.Flush()

	x := "sh /opt/gopath/src/github.com/hyperledger/fabric/peer/closeauction.sh"
	err := exe_cmd(x)
	if err != nil {
		fmt.Println("%s", err)
	}

	err = exe_cmd("rm /opt/gopath/src/github.com/hyperledger/fabric/peer/closeauction.sh")
	if err != nil {
		fmt.Println("%s", err)
	}

	fmt.Println("Kicking off CloseAuction", argStr)
	return nil
}

func exe_cmd(cmd string) error {

	fmt.Println("command :  ", cmd)
	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]

	_, err := exec.Command(head, parts...).CombinedOutput()
	if err != nil {
		fmt.Println("%s", err)
	}
	return err
}

//////////////////////////////////////////////////////////////////////////
// Close Open Auctions
// 1. Read OpenAucTable
// 2. Compare now with expiry time with now
// 3. If now is > expiry time call CloseAuction
//////////////////////////////////////////////////////////////////////////

/*func CloseOpenAuctions(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	rows, err := GetListOfOpenAucs(stub, "AucOpenTable", []string{"2016"})
	if err != nil {
		return nil, fmt.Errorf("GetListOfOpenAucs operation failed. Error marshaling JSON: %s", err)
	}

	tlist := make([]AuctionRequest, len(rows))
	err = json.Unmarshal([]byte(rows), &tlist)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	for i := 0; i < len(tlist); i++ {
		ar := tlist[i]
		if err != nil {
			fmt.Println("CloseOpenAuctions() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetListOfOpenAucs() operation failed. %s", err)
		}

		fmt.Println("CloseOpenAuctions() ", ar)

		// Compare Auction Times
		if tCompare(time.Now().Format("2006-01-02 15:04:05"), ar.CloseDate) == false {

			// Request Closing Auction
			_, err := CloseAuction(stub, "CloseAuction", []string{ar.AuctionID})
			if err != nil {
				fmt.Println("CloseOpenAuctions() Failed : Ummarshall error")
				return nil, fmt.Errorf("GetListOfOpenAucs() operation failed. %s", err)
			}
		}
	}

	return rows, nil
}*/

//////////////////////////////////////////////////////////////////////////
// Close the Auction
// This is invoked by OpenAuctionForBids
// which kicks-off a go-routine timer for the duration of the auction
// When the timer expires, it creates a shell script to CloseAuction() and triggers it
// This function can also be invoked via CLI - the intent was to close as and when I implement BuyItNow()
// CloseAuction
// - Sets the status of the Auction to "CLOSED"
// - Removes the Auction from the Open Auction list (AucOpenTable)
// - Retrieves the Highest Bid and creates a Transaction
// - Posts The Transaction
//
// To invoke from Command Line via CLI or REST API
// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "CloseAuction", "Args": ["1111", "AUCREQ"]}'
// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "CloseAuction", "Args": ["1111", "AUCREQ"]}'
//
//////////////////////////////////////////////////////////////////////////

/*func CloseAuction(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	// Close The Auction -  Fetch Auction Object
	Avalbytes, err := QueryLedger(stub, "AuctionTable", []string{args[0], "AUCREQ"})
	if err != nil {
		fmt.Println("CloseAuction(): Auction Object Retrieval Failed ")
		return nil, errors.New("CloseAuction(): Auction Object Retrieval Failed ")
	}

	aucR, err := JSONtoAucReq(Avalbytes)
	if err != nil {
		fmt.Println("CloseAuction(): Auction Object Unmarshalling Failed ")
		return nil, errors.New("CloseAuction(): Auction Object UnMarshalling Failed ")
	}

	//  Update Auction Status
	aucR.Status = "CLOSED"
	fmt.Println("CloseAuction(): UpdateAuctionStatus() successful ", aucR)

	Avalbytes, err = UpdateAuctionStatus(stub, "AuctionTable", aucR)
	if err != nil {
		fmt.Println("CloseAuction(): UpdateAuctionStatus() Failed ")
		return nil, errors.New("CloseAuction(): UpdateAuctionStatus() Failed ")
	}

	// Remove the Auction from Open Bucket
	keys := []string{"2016", aucR.AuctionID}
	err = DeleteFromLedger(stub, "AucOpenTable", keys)
	if err != nil {
		fmt.Println("CloseAuction(): DeleteFromLedger(AucOpenTable) Failed ")
		return nil, errors.New("CloseAuction(): DeleteFromLedger(AucOpenTable) Failed ")
	}

	fmt.Println("CloseAuction(): Proceeding to process the highest bid ")

	// Process Final Bid - Turn it into a Transaction
	Avalbytes, err = GetHighestBid(stub, "GetHighestBid", []string{args[0]})
	if Avalbytes == nil {
		fmt.Println("CloseAuction(): No bids available, no change in Item Status - PostTransaction() Completed Successfully ")
		return Avalbytes, nil
	}

	if err != nil {
		fmt.Println("CloseAuction(): No bids available, error encountered - PostTransaction() failed ")
		return nil, err
	}

	bid, _ := JSONtoBid(Avalbytes)
	fmt.Println("CloseAuction(): Proceeding to process the highest bid ", bid)
	tran := BidtoTransaction(bid)
	fmt.Println("CloseAuction(): Converting Bid to tran ", tran)

	// Process the last bid once Time Expires
	tranArgs := []string{tran.AuctionID, tran.RecType, tran.ItemID, tran.TransType, tran.UserId, tran.TransDate, tran.HammerTime, tran.HammerPrice, tran.Details}
	fmt.Println("CloseAuction(): Proceeding to process the  Transaction ", tranArgs)

	Avalbytes, err = PostTransaction(stub, "PostTransaction", tranArgs)
	if err != nil {
		fmt.Println("CloseAuction(): PostTransaction() Failed ")
		return nil, errors.New("CloseAuction(): PostTransaction() Failed ")
	}
	fmt.Println("CloseAuction(): PostTransaction() Completed Successfully ")
	return Avalbytes, nil
}*/

////////////////////////////////////////////////////////////////////////////////////////////
// Buy It Now
// Rules:
// If Buy IT Now Option is available then a Buyer has the option to buy the ITEM
// before the bids exceed BuyITNow Price . Normally, The application should take of this
// at the UI level and this chain-code assumes application has validated that
////////////////////////////////////////////////////////////////////////////////////////////

func BuyItNow(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	// Process Final Bid - Turn it into a Transaction
	Avalbytes, err := GetHighestBid(stub, "GetHighestBid", []string{args[0]})
	hBidFlag := true
	if Avalbytes == nil {
		fmt.Println("BuyItNow(): No bids available, no change in Item Status - PostTransaction() Completed Successfully ")
		hBidFlag = false
	}

	if err != nil {
		fmt.Println("BuyItNow(): No bids available, error encountered - PostTransaction() failed ")
		hBidFlag = false
	}

	//If there are some bids then do validations
	if hBidFlag == true {
		bid, err := JSONtoBid(Avalbytes)
		if err != nil {
			return nil, errors.New("BuyItNow() : JSONtoBid Error")
		}

		// Check if BuyItNow Price > Highest Bid so far
		binP, err := strconv.Atoi(args[5])
		if err != nil {
			return nil, errors.New("BuyItNow() : Invalid BuyItNow Price")
		}

		hbP, err := strconv.Atoi(bid.BidPrice)
		if err != nil {
			return nil, errors.New("BuyItNow() : Invalid Highest Bid Price")
		}

		if hbP > binP {
			return nil, errors.New("BuyItNow() : Highest Bid Price > BuyItNow Price - BuyItNow Rejected")
		}
	}

	// Close The Auction -  Fetch Auction Object
	Avalbytes, err = QueryLedger(stub, "AuctionTable", []string{args[0], "AUCREQ"})
	if err != nil {
		fmt.Println("BuyItNow(): Auction Object Retrieval Failed ")
		return nil, errors.New("BuyItNow(): Auction Object Retrieval Failed ")
	}

	aucR, err := JSONtoAucReq(Avalbytes)
	if err != nil {
		fmt.Println("BuyItNow(): Auction Object Unmarshalling Failed ")
		return nil, errors.New("BuyItNow(): Auction Object UnMarshalling Failed ")
	}

	//  Update Auction Status
	aucR.Status = "CLOSED"
	fmt.Println("BuyItNow(): UpdateAuctionStatus() successful ", aucR)

	Avalbytes, err = UpdateAuctionStatus(stub, "AuctionTable", aucR)
	if err != nil {
		fmt.Println("BuyItNow(): UpdateAuctionStatus() Failed ")
		return nil, errors.New("BuyItNow(): UpdateAuctionStatus() Failed ")
	}

	// Remove the Auction from Open Bucket
	keys := []string{"2016", aucR.AuctionID}
	err = DeleteFromLedger(stub, "AucOpenTable", keys)
	if err != nil {
		fmt.Println("BuyItNow(): DeleteFromLedger(AucOpenTable) Failed ")
		return nil, errors.New("BuyItNow(): DeleteFromLedger(AucOpenTable) Failed ")
	}

	fmt.Println("BuyItNow(): Proceeding to process the highest bid ")

	// Convert the BuyITNow to a Bid type struct
	/*buyItNowBid, err := CreateBidObject(args[0:])
	if err != nil {
		return nil, err
	}*/

	// Reject the offer if the Buyer Information Is not Valid or not registered on the Block Chain
	buyerInfo, err := ValidateMember(stub, args[4])
	fmt.Println("Buyer information  ", buyerInfo, args[4])
	if err != nil {
		fmt.Println("BuyItNow() : Failed Buyer not registered on the block-chain ", args[4])
		return nil, err
	}

	// tran := BidtoTransaction(buyItNowBid)
	// fmt.Println("BuyItNow(): Converting Bid to tran ", tran)

	// Process the buy-it-now offer
	// tranArgs := []string{tran.AuctionID, tran.RecType, tran.ItemID, tran.TransType, tran.UserId, tran.TransDate, tran.HammerTime, tran.HammerPrice, tran.Details}
	// fmt.Println("BuyItNow(): Proceeding to process the  Transaction ", tranArgs)

	// Avalbytes, err = PostTransaction(stub, "PostTransaction", tranArgs)
	if err != nil {
		fmt.Println("BuyItNow(): PostTransaction() Failed ")
		return nil, errors.New("CloseAuction(): PostTransaction() Failed ")
	}
	fmt.Println("BuyItNow(): PostTransaction() Completed Successfully ")
	return Avalbytes, nil
}

//////////////////////////////////////////////////////////////////////////
// Update the Auction Object
// This function updates the status of the auction
// from INIT to OPEN to CLOSED
//////////////////////////////////////////////////////////////////////////

func UpdateAuctionStatus(stub shim.ChaincodeStubInterface, tableName string, ar AuctionRequest) ([]byte, error) {

	buff, err := AucReqtoJSON(ar)
	if err != nil {
		fmt.Println("UpdateAuctionStatus() : Failed Cannot create object buffer for write : ", ar.AuctionID)
		return nil, errors.New("UpdateAuctionStatus(): Failed Cannot create object buffer for write : " + ar.AuctionID)
	}

	// Update the ledger with the Buffer Data
	keys := []string{ar.AuctionID, ar.ItemID}
	err = ReplaceLedgerEntry(stub, "AuctionTable", keys, buff)
	if err != nil {
		fmt.Println("UpdateAuctionStatus() : write error while inserting record\n")
		return buff, err
	}
	return buff, err
}
