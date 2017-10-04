package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

var dispatchOrderIndexstr = "_dispatchOrderindex"

//==============================================================================================================================
//	 Status types - contract lifecycle is broken down into 5 statuses, this is part of the business logic to determine what can
//					be done to the vehicle at points in it's lifecycle
//==============================================================================================================================
const STATE_OBD_REQUEST_CREATED = 0
const STATE_READY_FOR_DISPATCH = 1
const STATE_ARRIVAL_OF_TRANSPORTER = 2
const STATE_READY_FOR_SHIPMENT = 3
const STATE_IN_TRANSIT = 4
const STATE_SHIPMENT_DELIVERED = 5
const STATE_AMENDED = 6
const STATE_DROPPED = 7
const STATE_VOUCHER_CREATED = 8
const STATE_VOUCHER_VALIDATED = 9
const STATE_INVOICE_VALIDATED = 10
const STATE_INVOICE_GENERATED = 11

// DispatchOrderObject struct
type DispatchOrderObject struct {
	DispatchOrderID                string `json:"dispatchOrderId"`
	Stage                          string `json:"stage"`
	Customer                       string `json:"customer"`
	Transporter                    string `json:"transporter"`
	Seller                         string `json:"seller"`
	AssetIDs                       string `json:"assetIDs"`
	AsnNumber                      string `json:"asnNumber"`
	Source                         string `json:"source"`
	ShipmentType                   string `json:"shipmentType"`
	ContractType                   string `json:"contractType"`
	DeliveryTerm                   string `json:"deliveryTerm"`
	DispatchDate                   string `json:"dispatchDate"`
	TransporterRef                 string `json:"transporterRef"`
	LoadingType                    string `json:"loadingType"` //ource //destination //
	VehicleType                    string `json:"vehicleType"`
	Weight                         string `json:"weight"`
	Consignment                    string `json:"consignment"`
	Quantity                       string `json:"quantity"`
	PartNumber                     string `json:"partNumber"`
	PartName                       string `json:"partName"`
	OrderRefNum                    string `json:"orderRefNum"`
	CreatedOn                      string `json:"createdOn"`
	DocumentID1                    string `json:"documentID1"`
	DocumentID2                    string `json:"documentID2"`
	DocumentID3                    string `json:"documentID3"`
	DocumentID4                    string `json:"documentID4"`
	DropDescription                string `json:"dropDescription"`
	Deliverydescription            string `json:"deliverydescription"`
	InTransitDisptachOfficerSigned string `json:"inTransitDisptachOfficerSigned"`
	InTransitTransporterSigned     string `json:"inTransitTransporterSigned"`
	TransactionDescription         string `json:"transactionDescription"`
	TimeStamp                      string `json:"timeStamp"`
}

type AssetObject struct {
	AssetID            string `json:"assetId"`
	PartNumber         string `json:"partNumber"`
	PartDescription    string `json:"partDescription"`
	Owner              string `json:"owner"`
	Stage              string `json:"stage"`
	BatchNumer         string `json:"batchNumer"`
	ManufactureDate    string `json:"manufactureDate"`
	Itchs              string `json:"itchs"`
	ExciseChaperNumber string `json:"exciseChaperNumber"`
	OrderID            string `json:"orderId"`
}

type DocumentObject struct {
	DocumentID     string `json:"documentId"`
	DocumentName   string `json:"documentName"`
	DocumentType   string `json:"documentType"`
	DocumentString string `json:"documentString"`
	CreatedON      string `json:"createdOn"`
}

type TransactionHistoryObject struct {
	OrderID                string `json:"orderId"`
	Stage                  string `json:"stage"`
	Timestamp              string `json:"timestamp"`
	User                   string `json:"user"`
	TransactionDescription string `json:"transactionDescription"`
}

// DispatchOrderObject struct
type VoucherObject struct {
	VoucherID                      string `json:"voucherOrderID"`
	DispatchOrderID                string `json:"dispatchOrderId"`
	Stage                          string `json:"stage"`
	Customer                       string `json:"customer"`
	Transporter                    string `json:"transporter"`
	Seller                         string `json:"seller"`
	AssetIDs                       string `json:"assetIDs"`
	AsnNumber                      string `json:"asnNumber"`
	Source                         string `json:"source"`
	ShipmentType                   string `json:"shipmentType"`
	ContractType                   string `json:"contractType"`
	DeliveryTerm                   string `json:"deliveryTerm"`
	DispatchDate                   string `json:"dispatchDate"`
	TransporterRef                 string `json:"transporterRef"`
	LoadingType                    string `json:"loadingType"` //ource //destination //
	VehicleType                    string `json:"vehicleType"`
	Weight                         string `json:"weight"`
	Consignment                    string `json:"consignment"`
	Quantity                       string `json:"quantity"`
	PartNumber                     string `json:"partNumber"`
	PartName                       string `json:"partName"`
	OrderRefNum                    string `json:"orderRefNum"`
	CreatedOn                      string `json:"createdOn"`
	DocumentID1                    string `json:"documentID1"`
	DocumentID2                    string `json:"documentID2"`
	DocumentID3                    string `json:"documentID3"`
	DocumentID4                    string `json:"documentID4"`
	DropDescription                string `json:"dropDescription"`
	Deliverydescription            string `json:"deliverydescription"`
	InTransitDisptachOfficerSigned string `json:"inTransitDisptachOfficerSigned"`
	InTransitTransporterSigned     string `json:"inTransitTransporterSigned"`
	TransactionDescription         string `json:"transactionDescription"`
	TimeStamp                      string `json:"timeStamp"`
	Amount                         string `json:"amount"`
}

type InvoiceObject struct {
	InvoiceID   string `json:"InvoiceID"`
	VoucherList string `json:"VoucherList"`
	Stage       string `json:"stage"`
	Amount      string `json:"amount"`
}

var tables = []string{"AssetTable", "TransactionHistory", "DocumentTable", "VoucherTable", "InvoiceTable"}

// GetNumberOfKeys - Gets the number of keys for the table
func GetNumberOfKeys(tname string) int {
	TableMap := map[string]int{
		"AssetTable":         3,
		"TransactionHistory": 3,
		"DocumentTable":      3,
		"VoucherTable":       3,
		"InvoiceTable":       4, //"invoice","invoiceIds","stringof dispatch orders",buff -"amount"
	}
	return TableMap[tname]
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

// Init initializes the chain and three tables - one for asset,one for transaction history and other for documents
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	fmt.Println("Application Init")
	var err error

	for _, val := range tables {
		err = stub.DeleteTable(val)
		if err != nil {
			return nil, fmt.Errorf("Init(): DeleteTable of %s  Failed ", val)
		}
		err = InitLedger(stub, val)
		if err != nil {
			return nil, fmt.Errorf("Init(): InitLedger of %s  Failed ", val)
		}
	}

	fmt.Println("Init() Initialization Complete  : ", args)
	return []byte("Init(): Initialization Complete"), nil
}

// InitLedger - Initializes the tables
func InitLedger(stub shim.ChaincodeStubInterface, tableName string) error {

	// Generic Table Creation Function - requires Table Name and Table Key Entry
	// Create Table - Get number of Keys the tables supports
	// This version assumes all Keys are String and the Data is Bytes

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

// Invoke is our entry point to invoke a chaincode function
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("invoke is running " + function)

	if function == "createDispatchOrder" {
		return t.createDispatchOrder(stub, args)
	} else if function == "updateDispatchOrder" {
		return t.updateDispatchOrder(stub, args)
	} else if function == "createAsset" {
		return t.invokeAsset(stub, args)
	} else if function == "mapAsset" {
		return t.mapAsset(stub, args)
	} else if function == "createDocument" {
		return t.invokeDocument(stub, args)
	} else if function == "createVoucher" {
		return t.createVoucher(stub, args)
	} else if function == "updateVoucher" {
		return t.updateVoucher(stub, args)
	} else if function == "createInvoice" {
		return t.createInvoice(stub, args)
	} else if function == "validateInvoice" {
		return t.validateInvoice(stub, args)
	}
	fmt.Println("invoke did not find func: " + function) //error
	return nil, errors.New("Received unknown function invocation: " + function)
}

// Query queries the hyperledger
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("query is running " + function)

	// Handle different functions
	if function == "keys" {
		return t.getAllKeys(stub, args)
	} else if function == "read" { //read a contract
		return t.read(stub, args)
	} else if function == "getAssets" { //read a contract
		return t.getAssets(stub, args)
	} else if function == "getAllDispatchOrdersLatest" { //read a contract
		return t.getAllDispatchOrdersLatest(stub, args)
	} else if function == "getDocuments" { //read a contract
		return t.getDocuments(stub, args)
	} else if function == "getHistory" { //read a contract
		return t.getHistory(stub, args)
	} else if function == "get_caller_data" {
		return t.check_affiliation(stub)
	} else if function == "getVouchers" { //read a contract
		return t.getVouchers(stub, args)
	} else if function == "getInvoice" { //read a contract
		return t.getListofInvoices(stub, args)
	}
	fmt.Println("query did not find func: " + function) //error
	return nil, errors.New("Received unknown function query " + function)
}

func (t *SimpleChaincode) createDispatchOrder(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var err error

	//convert the arguments into an Diapatch order Object
	dispatchObject, err := createDispatchOrderObject(args[0:])
	if err != nil {
		fmt.Println("createDispatchOrder(): Cannot create dispatch object ")
		return nil, errors.New("createDispatchOrder(): Cannot create dipatch object")
	}

	// check if the DispatchOrder already exists
	contractAsBytes, err := stub.GetState(dispatchObject.DispatchOrderID)
	if err != nil {
		fmt.Println("createDispatchOrder() : failed to get contract")
		return nil, errors.New("Failed to get dispatchOrder")
	}
	if contractAsBytes != nil {
		fmt.Println("initContract() : contract already exists for ", dispatchObject.DispatchOrderID)
		jsonResp := "{\"Error\":\"Failed - contract already exists " + dispatchObject.DispatchOrderID + "\"}"
		return nil, errors.New(jsonResp)
	}

	buff, err := doToJSON(dispatchObject)
	if err != nil {
		errorStr := "initContract() : Failed Cannot create object buffer for write : " + args[0]
		fmt.Println(errorStr)
		return nil, errors.New(errorStr)
	}
	fmt.Println("createDispatchOrder() : buffer", buff)
	err = stub.PutState(args[0], buff)
	if err != nil {
		fmt.Println("initContract() : write error while inserting record\n")
		return nil, errors.New("initContract() : write error while inserting record : " + err.Error())
	}

	transactionTime := time.Now().Format("2006-01-02 15:04:05")
	xy, err3 := stub.GetCallerMetadata()
	if err3 != nil {
		fmt.Println(err3)
		return nil, err3
	}
	fmt.Println(xy)
	user := string(xy)
	fmt.Println("user is : ", user)
	//make an entry into transaction history table
	TransactionHistoryObject := TransactionHistoryObject{dispatchObject.DispatchOrderID, dispatchObject.Stage, transactionTime, user, dispatchObject.TransactionDescription}
	buffer, err := TRtoJSON(TransactionHistoryObject)
	if err != nil {
		fmt.Println("initContract() : Failed to convert transaction history to bytes\n")
		return nil, errors.New("initContract() : Failed to convert transaction history to bytes : " + err.Error())
	}
	keys := []string{"transaction", dispatchObject.DispatchOrderID, time.Now().Format("2006-01-02 15:04:05")}
	err = UpdateLedger(stub, "TransactionHistory", keys, buffer)
	//if err != nil {
	//	fmt.Println("initContract() : write error while inserting record\n")
	//	return buff, err
	//}

	return nil, nil
}

// read function return value
func (t *SimpleChaincode) updateDispatchOrder(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var jsonResp string
	var err error

	if len(args) != 31 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3 args")
	}

	dispatchorderID := args[0]
	dispatchOrderAsbytes, err := stub.GetState(dispatchorderID)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + dispatchorderID + "\"}"
		return nil, errors.New(jsonResp)
	}
	dat, err := JSONtoArgs(dispatchOrderAsbytes)
	if err != nil {
		return nil, errors.New("unable to convert jsonToArgs for" + dispatchorderID)
	}
	fmt.Println(dat)
	fmt.Println(dat["dispatchOrderId"])

	updatedDispatchOrder := DispatchOrderObject{dat["dispatchOrderId"], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], args[13], args[14], args[15], args[16], args[17], args[18], args[19], args[20], args[21], args[22], args[23], args[24], args[25], args[26], args[27], args[28], args[29], args[30], time.Now().Format("20060102150405")}

	buff, err := doToJSON(updatedDispatchOrder)
	if err != nil {
		errorStr := "updateDispatchOrder() : Failed Cannot create object buffer for write : " + args[0]
		fmt.Println(errorStr)
		return nil, errors.New(errorStr)
	}
	err = stub.PutState(dat["dispatchOrderId"], buff)
	if err != nil {
		fmt.Println("updateDispatchOrder() : write error while inserting record\n")
		return nil, errors.New("updateDispatchOrder() : write error while inserting record : " + err.Error())
	}

	transactionTime := time.Now().Format("2006-01-02 15:04:05")
	xy, err3 := stub.GetCallerMetadata()
	if err3 != nil {
		fmt.Println(err3)
		return nil, err3
	}
	fmt.Println(xy)
	user := string(xy)
	fmt.Println("user is : ", user)
	//make an entry into transaction history table
	TransactionHistoryObject := TransactionHistoryObject{updatedDispatchOrder.DispatchOrderID, updatedDispatchOrder.Stage, transactionTime, user, updatedDispatchOrder.TransactionDescription}
	buffer, err := TRtoJSON(TransactionHistoryObject)
	if err != nil {
		fmt.Println("updateDispatchOrder() : Failed to convert transaction history to bytes\n")
		return nil, errors.New("updateDispatchOrder() : Failed to convert transaction history to bytes : " + err.Error())
	}
	keys := []string{"transaction", updatedDispatchOrder.DispatchOrderID, time.Now().Format("2006-01-02 15:04:05")}
	err = UpdateLedger(stub, "TransactionHistory", keys, buffer)
	return nil, nil
}

// read - query function to read key/value pair
func (t *SimpleChaincode) read(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var key, jsonResp string
	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting name of the key to query")
	}

	key = args[0]
	valueAsbytes, err := stub.GetState(key)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + key + "\"}"
		return nil, errors.New(jsonResp)
	}
	fmt.Println("read contract output ", valueAsbytes)

	return valueAsbytes, nil
}

func (t *SimpleChaincode) getAllKeys(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	if len(args) < 2 {
		return nil, errors.New("put operation must include two arguments, a key and value")
	}

	startKey := args[0]
	endKey := args[1]

	keysIter, err := stub.RangeQueryState(startKey, endKey)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("keys operation failed. Error accessing state: %s", err))
	}
	defer keysIter.Close()
	var keys []string
	for keysIter.HasNext() {
		response, _, iterErr := keysIter.Next()
		if iterErr != nil {
			return nil, errors.New(fmt.Sprintf("keys operation failed. Error accessing state: %s", err))
		}
		keys = append(keys, response)
	}

	for key, value := range keys {
		fmt.Printf("key %d contains %s\n", key, value)
	}

	jsonKeys, err := json.Marshal(keys)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("keys operation failed. Error accessing state: %s", err))
	}

	return jsonKeys, nil
}

func (t *SimpleChaincode) getAllDispatchOrdersLatest(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var jsonResp string
	var valAsBytes []byte
	startKey := "1A"
	endKey := "Z*"

	keysIter, err := stub.RangeQueryState(startKey, endKey)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("keys operation failed. Error accessing state: %s", err))
	}
	defer keysIter.Close()
	var keys []string
	for keysIter.HasNext() {
		response, _, iterErr := keysIter.Next()
		if iterErr != nil {
			return nil, errors.New(fmt.Sprintf("keys operation failed. Error accessing state: %s", err))
		}
		keys = append(keys, response)
	}

	for key, value := range keys {
		fmt.Printf("key %d contains %s\n", key, value)
		valueAsbytes, err := stub.GetState(value)
		if err != nil {
			jsonResp = "{\"Error\":\"Failed to get state for " + value + "\"}"
			return nil, errors.New(jsonResp)
		}
		fmt.Println("read contract output ", valueAsbytes)
		valAsBytes = append(valAsBytes, valueAsbytes...)
	}
	fmt.Println("read all keys output ", valAsBytes)

	//fmt.Println("List of Open Auctions : ", jsonRows)
	return valAsBytes, nil
}

// CreateContractObject creates an contract
func createDispatchOrderObject(args []string) (DispatchOrderObject, error) {
	// S001 LHTMO bosch
	var err error
	var myDispatchOrder DispatchOrderObject

	// Check there are 31 Arguments provided as per the the struct, time is computed
	if len(args) != 31 {
		fmt.Println("CreateDispatchOrderObject(): Incorrect number of arguments. Expecting 31 ")
		return myDispatchOrder, errors.New("CreateDispatchOrderObject(): Incorrect number of arguments. Expecting 31 ")
	}

	//check whether the dispatch order already exists
	myDispatchOrder = DispatchOrderObject{args[0], strconv.Itoa(STATE_OBD_REQUEST_CREATED), args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], args[13], args[14], args[15], args[16], args[17], args[18], args[19], args[20], args[21], args[22], args[23], args[24], args[25], args[26], args[27], args[28], args[29], args[30], time.Now().Format("20060102150405")}
	if err != nil {
		fmt.Println(err)
		return myDispatchOrder, err
	}
	fmt.Println("CreateDispatchOrderObject(): dispatch Object created: ", myDispatchOrder)
	return myDispatchOrder, nil
}

// doToJSON Converts an dispatch Object to a JSON String
func doToJSON(c DispatchOrderObject) ([]byte, error) {
	cjson, err := json.Marshal(c)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println("dispatch object as bytes ", cjson)
	return cjson, nil
}

// doToJSON Converts an dispatch Object to a JSON String
func voToJSON(c VoucherObject) ([]byte, error) {
	cjson, err := json.Marshal(c)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println("dispatch object as bytes ", cjson)
	return cjson, nil
}

// JSON To args[] - return a map of the JSON string
func JSONtoArgs(Avalbytes []byte) (map[string]string, error) {

	var data map[string]string

	if err := json.Unmarshal(Avalbytes, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// invokes an asset into the table
func (t *SimpleChaincode) invokeAsset(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	assetObject, err := CreateAssetObject(args[0:])
	if err != nil {
		fmt.Println("invokeAsset(): Cannot create item object \n")
		return nil, err
	}

	/*// Check if the Owner ID specified is registered and valid */
	// Convert Item Object to JSON
	fmt.Println("assetObject is", assetObject)
	buff, err := ARtoJSON(assetObject)
	fmt.Println("buff is ", buff)
	if err != nil {
		fmt.Println("invokeAsset() : Failed Cannot create object buffer for write : ", args[0])
		return nil, errors.New("invokeAsset(): Failed Cannot create object buffer for write : " + args[0])
	} else {
		// Update the table with the Buffer Data
		keys := []string{"asset", assetObject.AssetID, assetObject.Owner}
		fmt.Println("invokeAsset() keys are :", keys)
		err = UpdateLedger(stub, "AssetTable", keys, buff)
		if err != nil {
			fmt.Println("invokeAsset() : write error while inserting record\n")
			return buff, err
		}
		return nil, nil
	}
}

func (t *SimpleChaincode) createVoucher(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var jsonResp string
	voucherObject, err := CreateVoucherObject(args[0:])
	if err != nil {
		fmt.Println("createVoucher(): Cannot create voucher object \n")
		return nil, err
	}
	fmt.Println("voucherObject is", voucherObject)

	dispatchObject, err := createDispatchOrderObject(args[0:])
	if err != nil {
		fmt.Println("createVoucher(): Cannot create dispatch object \n")
		return nil, err
	}

	fmt.Println("dispatchObject is", dispatchObject)
	dispatchObject.Stage = strconv.Itoa(STATE_VOUCHER_CREATED)
	dispatchOrderBuffer, err := doToJSON(dispatchObject)
	if err != nil {
		fmt.Println("createVoucher() : Failed Cannot create dispatch object buffer for write : ", args[0])
		return nil, errors.New("createVoucher(): Failed Cannot create dispatch object buffer for write : " + args[0])
	}
	fmt.Println("dispatch order buff is ", dispatchOrderBuffer)

	// Convert Item Object to JSON
	voucherObject.Stage = strconv.Itoa(STATE_VOUCHER_CREATED)
	buff, err := voToJSON(voucherObject)
	fmt.Println("buff is ", buff)
	if err != nil {
		fmt.Println("createVoucher() : Failed Cannot create voucher object buffer for write : ", args[0])
		return nil, errors.New("createVoucher(): Failed Cannot create voucher object buffer for write : " + args[0])
	} else {
		// Update the voucher table with the Buffer Data and updatedDispatchOrder with recent stage voucher created
		keys := []string{"voucher", voucherObject.VoucherID, "invoice"}
		fmt.Println("createVoucher() keys are :", keys)
		err = UpdateLedger(stub, "VoucherTable", keys, buff)
		if err != nil {
			fmt.Println("createVoucher() : write error while inserting record\n")
			return buff, err
		}
		// adding stage into main contract
		dispatchOrderAsbytes, err := stub.GetState(voucherObject.DispatchOrderID)
		if err != nil {
			jsonResp = "{\"Error\":\"Failed to get state for " + voucherObject.DispatchOrderID + "\"}"
			return nil, errors.New(jsonResp)
		}
		if dispatchOrderAsbytes == nil {
			fmt.Println("createVoucher() : contract is null for", voucherObject.DispatchOrderID)
			jsonResp := "{\"Error\":\"Failed - contract is null for " + voucherObject.DispatchOrderID + "\"}"
			return nil, errors.New(jsonResp)
		}
		err = stub.PutState(voucherObject.DispatchOrderID, dispatchOrderBuffer)
		if err != nil {
			fmt.Println("createVoucher() : write error while inserting record\n")
			return nil, errors.New("createVoucher() : write error while inserting record : " + err.Error())
		}

		transactionTime := time.Now().Format("2006-01-02 15:04:05")
		xy, err3 := stub.GetCallerMetadata()
		if err3 != nil {
			fmt.Println(err3)
			return nil, err3
		}
		fmt.Println(xy)
		user := string(xy)
		fmt.Println("user is : ", user)
		//make an entry into transaction history table
		TransactionHistoryObject := TransactionHistoryObject{voucherObject.DispatchOrderID, voucherObject.Stage, transactionTime, user, voucherObject.TransactionDescription}
		buffer, err := TRtoJSON(TransactionHistoryObject)
		if err != nil {
			fmt.Println("createVoucher() : Failed to convert transaction history to bytes\n")
			return nil, errors.New("createVoucher() : Failed to convert transaction history to bytes : " + err.Error())
		}
		Trasactionkeys := []string{"transaction", voucherObject.DispatchOrderID, time.Now().Format("2006-01-02 15:04:05")}
		err = UpdateLedger(stub, "TransactionHistory", Trasactionkeys, buffer)
		return nil, nil
	}
}

func (t *SimpleChaincode) updateVoucher(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var jsonResp string
	voucherObject, err := CreateUpdatedVoucherObject(args[:])
	if err != nil {
		fmt.Println("createVoucher(): Cannot create voucher object \n")
		return nil, err
	}
	fmt.Println("voucherObject is", voucherObject)

	dispatchObject, err := createDispatchOrderObject(args[1:32])
	if err != nil {
		fmt.Println("createVoucher(): Cannot create dispatch object \n")
		return nil, err
	}

	fmt.Println("dispatchObject is", dispatchObject)
	dispatchObject.Stage = strconv.Itoa(STATE_VOUCHER_VALIDATED)
	dispatchOrderBuffer, err := doToJSON(dispatchObject)
	if err != nil {
		fmt.Println("createVoucher() : Failed Cannot create dispatch object buffer for write : ", args[0])
		return nil, errors.New("createVoucher(): Failed Cannot create dispatch object buffer for write : " + args[0])
	}
	fmt.Println("dispatch order buff is ", dispatchOrderBuffer)

	// Convert Item Object to JSON
	voucherObject.Stage = strconv.Itoa(STATE_VOUCHER_VALIDATED)
	buff, err := voToJSON(voucherObject)
	fmt.Println("buff is ", buff)
	if err != nil {
		fmt.Println("createVoucher() : Failed Cannot create voucher object buffer for write : ", args[0])
		return nil, errors.New("createVoucher(): Failed Cannot create voucher object buffer for write : " + args[0])
	} else {
		// Update the voucher table with the Buffer Data and updatedDispatchOrder with recent stage voucher created
		keys := []string{"voucher", voucherObject.VoucherID, "invoice"}
		fmt.Println("createVoucher() keys are :", keys)
		err = ReplaceRowInLedger(stub, "VoucherTable", keys, buff)
		if err != nil {
			fmt.Println("createVoucher() : write error while inserting record\n")
			return buff, err
		}
		// adding stage into main contract
		dispatchOrderAsbytes, err := stub.GetState(voucherObject.DispatchOrderID)
		if err != nil {
			jsonResp = "{\"Error\":\"Failed to get state for " + voucherObject.DispatchOrderID + "\"}"
			return nil, errors.New(jsonResp)
		}
		if dispatchOrderAsbytes == nil {
			fmt.Println("createVoucher() : contract is null for", voucherObject.DispatchOrderID)
			jsonResp := "{\"Error\":\"Failed - contract is null for " + voucherObject.DispatchOrderID + "\"}"
			return nil, errors.New(jsonResp)
		}
		err = stub.PutState(voucherObject.DispatchOrderID, dispatchOrderBuffer)
		if err != nil {
			fmt.Println("createVoucher() : write error while inserting record\n")
			return nil, errors.New("createVoucher() : write error while inserting record : " + err.Error())
		}

		transactionTime := time.Now().Format("2006-01-02 15:04:05")
		xy, err3 := stub.GetCallerMetadata()
		if err3 != nil {
			fmt.Println(err3)
			return nil, err3
		}
		fmt.Println(xy)
		user := string(xy)
		fmt.Println("user is : ", user)
		//make an entry into transaction history table
		TransactionHistoryObject := TransactionHistoryObject{voucherObject.DispatchOrderID, voucherObject.Stage, transactionTime, user, voucherObject.TransactionDescription}
		buffer, err := TRtoJSON(TransactionHistoryObject)
		if err != nil {
			fmt.Println("createVoucher() : Failed to convert transaction history to bytes\n")
			return nil, errors.New("createVoucher() : Failed to convert transaction history to bytes : " + err.Error())
		}
		Trasactionkeys := []string{"transaction", voucherObject.DispatchOrderID, time.Now().Format("2006-01-02 15:04:05")}
		err = UpdateLedger(stub, "TransactionHistory", Trasactionkeys, buffer)
		return nil, nil
	}
}

func (t *SimpleChaincode) mapAsset(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var jsonResp string
	orderID := args[0]
	fmt.Println("orderId is" + orderID)
	//update blockchain
	dispatchOrderAsbytes, err := stub.GetState(orderID)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + orderID + "\"}"
		return nil, errors.New(jsonResp)
	}
	dat, err := JSONtoArgs(dispatchOrderAsbytes)
	if err != nil {
		return nil, errors.New("unable to convert jsonToArgs for" + orderID)
	}
	fmt.Println(dat)
	assetIds := args[1]
	dat["assetIds"] = assetIds
	fmt.Println("dispatch order with assets as bytes is ", dat)
	dispatchOrderAsString, err := json.Marshal(dat)
	if err != nil {
		return nil, errors.New("mapAsset(): Failed to convert map into string : " + args[0])
	}
	fmt.Println("dispatch order as map converted into json string is ", dispatchOrderAsString)

	/*dispatchOrderWithAssetsAsBytes, err := GetBytes(dat)
	if err != nil {
		return nil, errors.New("mapAsset(): Failed Cannot create object buffer for write : " + args[0])
	}*/
	result := strings.Split(assetIds, ",")
	for i := range result {
		keys := []string{"asset", result[i]}
		fmt.Println(keys)
		assetObjectFromLedger, err := getAssetFromTable(stub, keys)
		if err != nil {
			return nil, fmt.Errorf("GetAssets() operation failed. Error marshaling JSON: %s", err)
		}
		assetObjectFromLedger.OrderID = orderID
		assetObjectFromLedger.Stage = "Mapped"
		buff, err := ARtoJSON(assetObjectFromLedger)
		fmt.Println("buff is ", buff)
		if err != nil {
			fmt.Println("mapAsset() : Failed Cannot create object buffer for write : ", args[0])
			return nil, errors.New("mapAsset(): Failed Cannot create object buffer for write : " + args[0])
		} else {
			// Update the table with the Buffer Data
			keys := []string{"asset", assetObjectFromLedger.AssetID, assetObjectFromLedger.Owner}
			fmt.Println("mapAsset() keys are :", keys)

			err = ReplaceRowInLedger(stub, "AssetTable", keys, buff)
			if err != nil {
				fmt.Println("invokeAsset() : write error while inserting record\n")
				return buff, err
			}
			//uypdate the block with added Assets
			err = stub.PutState(dat["dispatchOrderId"], dispatchOrderAsString)
			if err != nil {
				fmt.Println("updateDispatchOrder() : write error while inserting record\n")
				return nil, errors.New("updateDispatchOrder() : write error while inserting record : " + err.Error())
			}

		}
	}
	return []byte("Assets Mapped"), nil
}

func (t *SimpleChaincode) createInvoice(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var jsonResp string
	invoiceAmount := 0
	invoiceID := args[0]
	fmt.Println("invoiceID is" + invoiceID)
	voucherIds := args[1]
	result := strings.Split(voucherIds, ",")
	for i := range result {
		keys := []string{"voucher", result[i]}
		fmt.Println(keys)
		voucherObjectFromLedger, err := getVoucherFromTable(stub, keys)
		if err != nil {
			return nil, fmt.Errorf("createInvoice() operation failed. Error marshaling JSON: %s", err)
		}
		//fetching dispatch order from block
		dispatchOrderAsbytes, err := stub.GetState(result[i])
		if err != nil {
			jsonResp = "{\"Error\":\"Failed to get state for " + result[i] + "\"}"
			return nil, errors.New(jsonResp)
		}

		dat, err := JSONtoArgs(dispatchOrderAsbytes)
		if err != nil {
			return nil, errors.New("unable to convert jsonToArgs for" + result[i])
		}
		fmt.Println(dat)
		fmt.Println(dat["dispatchOrderId"])
		dat["stage"] = strconv.Itoa(STATE_INVOICE_GENERATED)
		fmt.Println("dispatch order with assets as bytes is ", dat)
		dispatchOrderAsString, err := json.Marshal(dat)
		if err != nil {
			return nil, errors.New("mapAsset(): Failed to convert map into string : " + args[0])
		}
		fmt.Println("dispatch order as map converted into json string is ", dispatchOrderAsString)

		//update voucher table data
		invoiceAmountAsInt, err := strconv.Atoi(voucherObjectFromLedger.Amount)
		if err != nil {
			return nil, errors.New("unable to convert jsonToArgs for" + result[i])
		}
		invoiceAmount = invoiceAmount + invoiceAmountAsInt
		fmt.Println("Invoice amount for", dat["dispatchOrderId"], "is", invoiceAmount)
		voucherObjectFromLedger.Stage = strconv.Itoa(STATE_INVOICE_GENERATED)
		buff, err := voToJSON(voucherObjectFromLedger)
		fmt.Println("buff is ", buff)
		if err != nil {
			fmt.Println("createInvoice() : Failed Cannot create object buffer for write : ", args[0])
			return nil, errors.New("createInvoice(): Failed Cannot create object buffer for write : " + args[0])
		} else {
			// Update the table with the Buffer Data
			keys := []string{"voucher", voucherObjectFromLedger.DispatchOrderID, voucherObjectFromLedger.Stage, invoiceID}
			fmt.Println("createInvoice() keys are :", keys)

			err = UpdateLedger(stub, "VoucherTable", keys, buff)
			if err != nil {
				fmt.Println("createInvoice() : write error while inserting record\n")
				return buff, err
			}

			//uypdate the block with added Assets
			err = stub.PutState(dat["dispatchOrderId"], dispatchOrderAsString)
			if err != nil {
				fmt.Println("updateDispatchOrder() : write error while inserting record\n")
				return nil, errors.New("updateDispatchOrder() : write error while inserting record : " + err.Error())
			}
		}
	}
	//create invoice table
	invoice := []string{invoiceID, voucherIds, strconv.Itoa(STATE_INVOICE_GENERATED), strconv.Itoa(invoiceAmount)}
	invoiceObject, err := CreateInvoiceObject(invoice)
	if err != nil {
		fmt.Println("invokeAsset(): Cannot create item object \n")
		return nil, err
	}

	fmt.Println("invoiceObject is", invoiceObject)
	buffInvoice, err := InvoicetoJSON(invoiceObject)
	fmt.Println("invoice buff is ", buffInvoice)

	transactionTime := time.Now().Format("2006-01-02 15:04:05")
	keys := []string{"invoice", invoiceID, strconv.Itoa(STATE_INVOICE_GENERATED), transactionTime}
	fmt.Println("createInvoice() keys are :", keys)
	err = UpdateLedger(stub, "InvoiceTable", keys, buffInvoice)
	if err != nil {
		fmt.Println("createInvoice() : write error while inserting record\n")
		return buffInvoice, err
	}

	return []byte("Invoice Created"), nil
}

func (t *SimpleChaincode) validateInvoice(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var jsonResp string

	// takes invoice and updatesinvoicetable
	if len(args) != 1 {
		fmt.Println("validateInvoice(): Incorrect number of arguments. Expecting 1 ")
		return []byte("Invoice validation failed"), errors.New("validateInvoice(): Incorrect number of arguments. Expecting 1 ")
	}
	invoiceID := args[0]
	InvoiceObjectFromLedger, err := getunValidatedInvoice(stub, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("validateInvoice() operation failed. Unable to fetch invoice from ledger: %s", err)
	}

	InvoiceObjectFromLedger.Stage = strconv.Itoa(STATE_INVOICE_VALIDATED)

	newInvoiceObjectbuff, err := InvoicetoJSON(InvoiceObjectFromLedger)
	fmt.Println("newInvoiceObjectbuff is ", newInvoiceObjectbuff)

	transactionTime := time.Now().Format("2006-01-02 15:04:05")
	keys := []string{"invoice", invoiceID, strconv.Itoa(STATE_INVOICE_VALIDATED), transactionTime}
	fmt.Println("createInvoice() keys are :", keys)
	err = UpdateLedger(stub, "InvoiceTable", keys, newInvoiceObjectbuff)
	if err != nil {
		fmt.Println("createInvoice() : write error while inserting record\n")
		return newInvoiceObjectbuff, err
	}

	//updates voucher table with state invoice validated
	// updates blockchain

	voucherIds := InvoiceObjectFromLedger.VoucherList
	result := strings.Split(voucherIds, ",")
	for i := range result {
		keys := []string{"voucher", result[i]}
		fmt.Println(keys)
		voucherObjectFromLedger, err := getVoucherFromTable(stub, keys)
		if err != nil {
			return nil, fmt.Errorf("createInvoice() operation failed. Error marshaling JSON: %s", err)
		}
		//fetching dispatch order from block
		dispatchOrderAsbytes, err := stub.GetState(result[i])
		if err != nil {
			jsonResp = "{\"Error\":\"Failed to get state for " + result[i] + "\"}"
			return nil, errors.New(jsonResp)
		}

		dat, err := JSONtoArgs(dispatchOrderAsbytes)
		if err != nil {
			return nil, errors.New("unable to convert jsonToArgs for" + result[i])
		}
		fmt.Println(dat)
		fmt.Println(dat["dispatchOrderId"])
		dat["stage"] = strconv.Itoa(STATE_INVOICE_VALIDATED)
		fmt.Println("dispatch order with assets as bytes is ", dat)
		dispatchOrderAsString, err := json.Marshal(dat)
		if err != nil {
			return nil, errors.New("mapAsset(): Failed to convert map into string : " + args[0])
		}
		fmt.Println("dispatch order as map converted into json string is ", dispatchOrderAsString)

		//update voucher table data
		voucherObjectFromLedger.Stage = strconv.Itoa(STATE_INVOICE_VALIDATED)
		buff, err := voToJSON(voucherObjectFromLedger)
		fmt.Println("buff is ", buff)
		if err != nil {
			fmt.Println("createInvoice() : Failed Cannot create object buffer for write : ", args[0])
			return nil, errors.New("createInvoice(): Failed Cannot create object buffer for write : " + args[0])
		} else {
			// Update the table with the Buffer Data
			keys := []string{"voucher", voucherObjectFromLedger.DispatchOrderID, voucherObjectFromLedger.Stage, invoiceID}
			fmt.Println("createInvoice() keys are :", keys)

			err = UpdateLedger(stub, "VoucherTable", keys, buff)
			if err != nil {
				fmt.Println("createInvoice() : write error while inserting record\n")
				return buff, err
			}

			//uypdate the block with added Assets
			err = stub.PutState(dat["dispatchOrderId"], dispatchOrderAsString)
			if err != nil {
				fmt.Println("updateDispatchOrder() : write error while inserting record\n")
				return nil, errors.New("updateDispatchOrder() : write error while inserting record : " + err.Error())
			}
		}
	}

	//transaction history
	return []byte("Invoice Validated"), nil

}

// CreateAssetObject creates an asset
func CreateAssetObject(args []string) (AssetObject, error) {
	// S001 LHTMO bosch
	// var err error to be
	var myAsset AssetObject

	// Check there are 3 Arguments provided as per the the struct
	if len(args) != 10 {
		fmt.Println("CreateAssetObject(): Incorrect number of arguments. Expecting 10 ")
		return myAsset, errors.New("CreateAssetObject(): Incorrect number of arguments. Expecting 10 ")
	}

	// Validate Serialno is an integer

	/*_, err = strconv.Atoi(args[0])
	if err != nil {
		fmt.Println("CreateAssetObject(): SerialNo should be an integer create failed! ")
		return myAsset, errors.New("CreateAssetbject(): SerialNo should be an integer create failed. ")
	}*/

	myAsset = AssetObject{args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9]}
	return myAsset, nil
}

func ARtoJSON(ast AssetObject) ([]byte, error) {

	ajson, err := json.Marshal(ast)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

func InvoicetoJSON(inv InvoiceObject) ([]byte, error) {

	ajson, err := json.Marshal(inv)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

// CreateAssetObject creates an asset
func CreateVoucherObject(args []string) (VoucherObject, error) {
	// S001 LHTMO bosch
	// var err error to be
	var myVoucher VoucherObject

	// Check there are 31 Arguments provided as per the the struct
	if len(args) != 31 {
		fmt.Println("CreateVoucherObject(): Incorrect number of arguments. Expecting 31 ")
		return myVoucher, errors.New("CreateVoucherObject(): Incorrect number of arguments. Expecting 31 ")
	}

	myVoucher = VoucherObject{args[0], args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], args[13], args[14], args[15], args[16], args[17], args[18], args[19], args[20], args[21], args[22], args[23], args[24], args[25], args[26], args[27], args[28], args[29], args[30], time.Now().Format("20060102150405"), "amount"}
	voucherAmount, err := CalculateVoucherAmount(myVoucher)
	if err != nil {
		fmt.Println(err)
		return myVoucher, err
	}

	myVoucher.Amount = strconv.Itoa(voucherAmount)
	return myVoucher, nil
}

func CreateUpdatedVoucherObject(args []string) (VoucherObject, error) {
	// S001 LHTMO bosch
	// var err error to be
	var myVoucher VoucherObject

	// Check there are 3 Arguments provided as per the the struct
	if len(args) != 34 {
		fmt.Println("CreateVoucherObject(): Incorrect number of arguments. Expecting 34 current args %d", len(args))
		return myVoucher, errors.New("CreateVoucherObject(): Incorrect number of arguments. Expecting 34 ")
	}

	myVoucher = VoucherObject{args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], args[13], args[14], args[15], args[16], args[17], args[18], args[19], args[20], args[21], args[22], args[23], args[24], args[25], args[26], args[27], args[28], args[29], args[30], args[31], time.Now().Format("20060102150405"), args[33]}
	return myVoucher, nil
}

// CreateAssetObject creates an asset
func CreateInvoiceObject(args []string) (InvoiceObject, error) {

	var myInvoice InvoiceObject

	if len(args) != 4 {
		fmt.Println("CreateInvoiceObject(): Incorrect number of arguments. Expecting 4 ")
		return myInvoice, errors.New("CreateInvoiceObject(): Incorrect number of arguments. Expecting 4 ")
	}

	myInvoice = InvoiceObject{args[0], args[1], args[2], args[3]}
	return myInvoice, nil
}

func CalculateVoucherAmount(voucherObject VoucherObject) (int, error) {

	weight, err := strconv.Atoi(voucherObject.Weight)
	if err != nil {
		return 0, errors.New("CalculateVoucherAmount: Unable to convert weight to int")

	}
	if voucherObject.LoadingType == "LTL" {
		if voucherObject.Customer == "Maruthi Pune" {
			return (2500 * weight), nil
		} else if voucherObject.Customer == "Ashok Leyland Hosur" {
			return (125 * weight), nil
		} else if voucherObject.Customer == "Ford Chennai" {
			return (1100 * weight), nil
		}
	} else if voucherObject.LoadingType == "FTL" && voucherObject.VehicleType == "16 Tonner" {
		if voucherObject.Customer == "Maruthi Pune" {
			return (2500 * 16), nil
		} else if voucherObject.Customer == "Ashok Leyland Hosur" {
			return (125 * 16), nil
		} else if voucherObject.Customer == "Ford Chennai" {
			return (1100 * 16), nil
		}
	} else if voucherObject.LoadingType == "FTL" && voucherObject.VehicleType == "21 Tonner" {
		if voucherObject.Customer == "Maruthi Pune" {
			return (2500 * 21), nil
		} else if voucherObject.Customer == "Ashok Leyland Hosur" {
			return (125 * 21), nil
		} else if voucherObject.Customer == "Ford Chennai" {
			return (1100 * 21), nil
		}
	}
	return 0, nil
}

func TRtoJSON(to TransactionHistoryObject) ([]byte, error) {

	ajson, err := json.Marshal(to)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

func UpdateLedger(stub shim.ChaincodeStubInterface, tableName string, keys []string, args []byte) error {

	fmt.Println("buffer is ", args)
	fmt.Println("keys is ", keys)

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
	fmt.Println("appending row is", row)
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

func ReplaceRowInLedger(stub shim.ChaincodeStubInterface, tableName string, keys []string, args []byte) error {

	fmt.Println("buffer is ", args)
	fmt.Println("keys is ", keys)

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
	fmt.Println("appending row is", row)
	ok, err := stub.ReplaceRow(tableName, row)
	if err != nil {
		return fmt.Errorf("UpdateLedger: InsertRow into "+tableName+" Table operation failed. %s", err)
	}
	if !ok {
		return errors.New("UpdateLedger: InsertRow into " + tableName + " Table failed. Row with given key " + keys[0] + " already exists")
	}

	fmt.Println("UpdateLedger: InsertRow into ", tableName, " Table operation Successful. ")
	return nil
}

func (t *SimpleChaincode) getAssets(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	rows, err := GetList(stub, "AssetTable", args)
	if err != nil {
		return nil, fmt.Errorf("GetAssets() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("AssetTable")

	tlist := make([]AssetObject, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoAR(ts)
		if err != nil {
			fmt.Println("GetAssets() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetAssets() operation failed. %s", err)
		}
		tlist[i] = ar
	}

	jsonRows, _ := json.Marshal(tlist)

	//fmt.Println("List of Open Auctions : ", jsonRows)
	return jsonRows, nil
}

func (t *SimpleChaincode) getListofInvoices(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	rows, err := GetList(stub, "InvoiceTable", args)
	if err != nil {
		return nil, fmt.Errorf("getInvoice() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("InvoiceTable")

	tlist := make([]InvoiceObject, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoInvoice(ts)
		if err != nil {
			fmt.Println("GetAssets() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetAssets() operation failed. %s", err)
		}
		tlist[i] = ar
	}
	jsonRows, _ := json.Marshal(tlist)
	return jsonRows, nil
}

func getunValidatedInvoice(stub shim.ChaincodeStubInterface, InvoiceID string) (InvoiceObject, error) {

	rows, err := GetList(stub, "InvoiceTable", []string{"invoice", InvoiceID, "9"})
	if err != nil {
		return InvoiceObject{}, fmt.Errorf("getInvoice() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("InvoiceTable")

	//tlist := make([]InvoiceObject, len(rows))
	//for i := 0; i < len(rows); i++ {
	ts := rows[0].Columns[nCol].GetBytes()
	ar, err := JSONtoInvoice(ts)
	if err != nil {
		fmt.Println("GetAssets() Failed : Ummarshall error")
		return InvoiceObject{}, fmt.Errorf("GetAssets() operation failed. %s", err)
	}
	//tlist[i] = ar
	//}
	return ar, nil
}

func getAssetFromTable(stub shim.ChaincodeStubInterface, args []string) (AssetObject, error) {

	rows, err := GetList(stub, "AssetTable", args)
	if err != nil {
		return AssetObject{}, fmt.Errorf("GetAssets() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("AssetTable")

	// tlist := make([]AssetObject, len(rows))
	//for i := 0; i < len(rows); i++ {
	ts := rows[0].Columns[nCol].GetBytes()
	ar, err := JSONtoAR(ts)
	if err != nil {
		fmt.Println("GetAssets() Failed : Ummarshall error")
		return AssetObject{}, fmt.Errorf("GetAssets() operation failed. %s", err)
	}
	// tlist[i] = ar
	//}

	return ar, nil
}

func getVoucherFromTable(stub shim.ChaincodeStubInterface, args []string) (VoucherObject, error) {

	rows, err := GetList(stub, "VoucherTable", args)
	if err != nil {
		return VoucherObject{}, fmt.Errorf("getInvoiceFromTable() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("VoucherTable")

	// tlist := make([]AssetObject, len(rows))
	//for i := 0; i < len(rows); i++ {
	ts := rows[0].Columns[nCol].GetBytes()
	ar, err := JSONtoVO(ts)
	if err != nil {
		fmt.Println("getInvoiceFromTable() Failed : Ummarshall error")
		return VoucherObject{}, fmt.Errorf("getInvoiceFromTable() operation failed. %s", err)
	}
	// tlist[i] = ar
	//}

	return ar, nil
}

func (t *SimpleChaincode) getVouchers(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	rows, err := GetList(stub, "VoucherTable", args)
	if err != nil {
		return nil, fmt.Errorf("GetAssets() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("VoucherTable")

	tlist := make([]VoucherObject, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoVO(ts)
		if err != nil {
			fmt.Println("GetAssets() Failed : Ummarshall error")
			return nil, fmt.Errorf("GetAssets() operation failed. %s", err)
		}
		tlist[i] = ar
	}

	jsonRows, _ := json.Marshal(tlist)
	return jsonRows, nil
}

func GetList(stub shim.ChaincodeStubInterface, tableName string, args []string) ([]shim.Row, error) {
	var columns []shim.Column

	fmt.Println("number of args is", len(args))
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

func JSONtoAR(data []byte) (AssetObject, error) {

	ar := AssetObject{}
	err := json.Unmarshal([]byte(data), &ar)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	return ar, err
}

func JSONtoInvoice(data []byte) (InvoiceObject, error) {

	ar := InvoiceObject{}
	err := json.Unmarshal([]byte(data), &ar)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	return ar, err
}

func JSONtoDO(data []byte) (DispatchOrderObject, error) {

	do := DispatchOrderObject{}
	err := json.Unmarshal([]byte(data), &do)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	return do, err
}

func JSONtoVO(data []byte) (VoucherObject, error) {

	do := VoucherObject{}
	err := json.Unmarshal([]byte(data), &do)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	return do, err
}

// invokes an asset into the table
func (t *SimpleChaincode) invokeDocument(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	documentObject, err := CreateDocumentObject(args[0:])
	if err != nil {
		fmt.Println("invokeDocument(): Cannot create item object \n")
		return nil, err
	}

	/*// Check if the Owner ID specified is registered and valid */
	// Convert Item Object to JSON
	fmt.Println("documentObject is", documentObject)
	buff, err := DOCtoJSON(documentObject)
	fmt.Println("buff is ", buff)
	if err != nil {
		fmt.Println("invokeDocument() : Failed Cannot create object buffer for write : ", args[0])
		return nil, errors.New("invokeDocument(): Failed Cannot create object buffer for write : " + args[0])
	} else {
		// Update the table with the Buffer Data
		keys := []string{"document", documentObject.DocumentID, time.Now().Format("20060102150405")}
		err = UpdateLedger(stub, "DocumentTable", keys, buff)
		if err != nil {
			fmt.Println("invokeDocument() : write error while inserting record")
			return buff, err
		}
		return nil, nil
	}
}

// CreateAssetObject creates an asset
func CreateDocumentObject(args []string) (DocumentObject, error) {
	// S001 LHTMO bosch
	// var err error to be
	var myDocument DocumentObject

	// Check there are 3 Arguments provided as per the the struct
	if len(args) != 4 {
		fmt.Println("CreateDocumentObject(): Incorrect number of arguments. Expecting 4 ")
		return myDocument, errors.New("CreateDocumentObject(): Incorrect number of arguments. Expecting 4 ")
	}

	myDocument = DocumentObject{args[0], args[1], args[2], args[3], time.Now().Format("20060102150405")}
	return myDocument, nil
}

func DOCtoJSON(doc DocumentObject) ([]byte, error) {

	djson, err := json.Marshal(doc)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return djson, nil
}

func JSONtoDOC(data []byte) (DocumentObject, error) {

	doc := DocumentObject{}
	err := json.Unmarshal([]byte(data), &doc)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
		return doc, err
	}

	return doc, nil
}

func JSONtoTX(data []byte) (TransactionHistoryObject, error) {

	tx := TransactionHistoryObject{}
	err := json.Unmarshal([]byte(data), &tx)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
		return tx, err
	}

	return tx, nil
}

func (t *SimpleChaincode) getDocuments(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	rows, err := GetList(stub, "DocumentTable", args)
	if err != nil {
		return nil, fmt.Errorf("getDocuments() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("DocumentTable")

	tlist := make([]DocumentObject, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoDOC(ts)
		if err != nil {
			fmt.Println("getDocuments() Failed : Ummarshall error")
			return nil, fmt.Errorf("getDocuments() operation failed. %s", err)
		}
		tlist[i] = ar
	}

	jsonRows, _ := json.Marshal(tlist)

	//fmt.Println("List of Open Auctions : ", jsonRows)
	return jsonRows, nil

}

func (t *SimpleChaincode) getHistory(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	rows, err := GetList(stub, "TransactionHistory", args)
	if err != nil {
		return nil, fmt.Errorf("getHistory() operation failed. Error marshaling JSON: %s", err)
	}

	nCol := GetNumberOfKeys("TransactionHistory")

	tlist := make([]TransactionHistoryObject, len(rows))
	for i := 0; i < len(rows); i++ {
		ts := rows[i].Columns[nCol].GetBytes()
		ar, err := JSONtoTX(ts)
		if err != nil {
			fmt.Println("getHistory() Failed : Ummarshall error")
			return nil, fmt.Errorf("getHistory() operation failed. %s", err)
		}
		tlist[i] = ar
	}

	jsonRows, _ := json.Marshal(tlist)

	//fmt.Println("List of Open Auctions : ", jsonRows)
	return jsonRows, nil

}

func (t *SimpleChaincode) check_affiliation(stub shim.ChaincodeStubInterface) ([]byte, error) {
	affiliation, err := stub.ReadCertAttribute("roles")
	fmt.Println("role is ", string(affiliation))

	if err != nil {
		return []byte(""), errors.New("Couldn't get attribute 'role'. Error: " + err.Error())
	}
	return affiliation, nil

}

func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
