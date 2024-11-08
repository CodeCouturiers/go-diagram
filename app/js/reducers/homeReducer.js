import * as AppConstants from '../constants/AppConstants';
import assignToEmpty from '../utils/assign';
import _ from 'underscore';
import Connection from '../utils/Connection';

const initialState = {
    packageData: {
        packages: [{
            name: 'loading...',
            files: [{
                name: 'loading...',
                structs: [],
            }],
        }],
        edges: []
    }
};


function clone(state) {
    return assignToEmpty(state, {});
}


function getFileData(state, file) {
    let packages = state.packageData.packages;
    let packageIndex = _.findIndex(packages, (pkg) => pkg.name === file.package);
    let files = packages[packageIndex].files;
    let fileIndex = _.findIndex(files, (f) => f.name === file.file);
    return {
        packageIndex,
        fileIndex,
    };
}

function getStructData(state, struct) {
    let {
        packageIndex,
        fileIndex,
    } = getFileData(state, struct);
    let structs = state.packageData.packages[packageIndex].files[fileIndex].structs;
    let structIndex = _.findIndex(structs, (fileStructs) => fileStructs.name === struct.name);
    return {
        packageIndex,
        fileIndex,
        structIndex,
    };
}

function homeReducer(state = initialState, action) {
    Object.freeze(state);
    let newState = clone(state);

    switch(action.type) {
        case AppConstants.SET_PACKAGE_DATA:
            newState.packageData = action.packageData;
            return newState;
        case AppConstants.CLEAR_LAYOUT:
            // Очистка layout, если необходимо
            return {
                ...newState,
                packageData: {
                    packages: [],
                    edges: []
                }
            };

        case AppConstants.UPDATE_FROM_FILE_CHANGE:
            // Обработка обновлений от изменений файла
            if (action.updatedData && action.updatedData.packages) {
                newState.packageData = action.updatedData;
                return newState;
            }
            console.error('Invalid updated data received:', action.updatedData);
            return state;


        case AppConstants.DELETE_STRUCT:
            let deleteStruct = getStructData(state, action.struct);
            newState.packageData.packages[deleteStruct.packageIndex].files[deleteStruct.fileIndex].structs.splice(deleteStruct.structIndex, 1);
            return newState;

        case AppConstants.CHANGE_STRUCT_NAME:
            let changeNameStruct = getStructData(state, action.struct);
            let updatedStruct = newState.packageData.packages[changeNameStruct.packageIndex].files[changeNameStruct.fileIndex].structs[changeNameStruct.structIndex];
            updatedStruct.name = action.struct.newName;
            return newState;

        case AppConstants.CHANGE_STRUCT_FIELD_NAME:
            let changeFieldNameStruct = getStructData(state, action.struct);
            let updatedField = newState.packageData.packages[changeFieldNameStruct.packageIndex].files[changeFieldNameStruct.fileIndex].structs[changeFieldNameStruct.structIndex].fields[action.struct.key];
            updatedField.name = action.struct.newFieldName;
            return newState;

        case AppConstants.CHANGE_STRUCT_FIELD_TYPE:
            let changeFieldTypeStruct = getStructData(state, action.struct);
            let updatedTypeField = newState.packageData.packages[changeFieldTypeStruct.packageIndex].files[changeFieldTypeStruct.fileIndex].structs[changeFieldTypeStruct.structIndex].fields[action.struct.key];
            updatedTypeField.type.literal = action.struct.newFieldType;
            return newState;

        case AppConstants.ADD_STRUCT_FIELD:
            let addFieldStruct = getStructData(state, action.struct);
            newState.packageData.packages[addFieldStruct.packageIndex].files[addFieldStruct.fileIndex].structs[addFieldStruct.structIndex].fields.push({
                name: '[name]',
                type: {
                    literal: '[type]',
                    structs: [],
                },
            });
            return newState;

        case AppConstants.REMOVE_STRUCT_FIELD:
            let removeFieldStruct = getStructData(state, action.struct);
            newState.packageData.packages[removeFieldStruct.packageIndex].files[removeFieldStruct.fileIndex].structs[removeFieldStruct.structIndex].fields.splice(action.struct.key, 1);
            return newState;

        case AppConstants.CHANGE_STRUCT_METHOD_NAME:
            let changeMethodNameStruct = getStructData(state, action.data);
            let updatedMethod = newState.packageData.packages[changeMethodNameStruct.packageIndex].files[changeMethodNameStruct.fileIndex].structs[changeMethodNameStruct.structIndex].methods[action.data.methodIndex];
            updatedMethod.name = action.data.newMethodName;
            return newState;

        case AppConstants.CHANGE_STRUCT_METHOD_RETURN_TYPE:
            let changeMethodReturnTypeStruct = getStructData(state, action.data);
            let updatedReturnTypeMethod = newState.packageData.packages[changeMethodReturnTypeStruct.packageIndex].files[changeMethodReturnTypeStruct.fileIndex].structs[changeMethodReturnTypeStruct.structIndex].methods[action.data.methodIndex];
            updatedReturnTypeMethod.returnType[action.data.typeIndex].literal = action.data.newReturnType;
            return newState;

        case AppConstants.ADD_STRUCT:
            let addStructFile = getFileData(state, action.file);
            newState.packageData.packages[addStructFile.packageIndex].files[addStructFile.fileIndex].structs.push({
                name: '[name]',
                fields: [],
            });
            return newState;

        default:
            return state;
    }
}

export default homeReducer;