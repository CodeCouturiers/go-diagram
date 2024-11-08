import React, { Component, createRef } from 'react';
import AutosizeInput from 'react-18-input-autosize';

// Constants
const STRUCT_MIN_WIDTH = 120;

const noop = () => {};

class Struct extends Component {
    constructor(props) {
        super(props);
        this.state = {
            name: props.name,
            fields: props.fields,
            methods: props.methods || [],
        };
        this.headerRef = createRef();
        this.fieldNameRefs = [];
        this.fieldTypeRefs = [];
        this.methodNameRefs = [];
        this.methodReturnRefs = [];
    }

    static defaultProps = {
        className: '',
        package: '',
        file: '',
        name: '',
        fields: [],
        methods: [],
        searchTerm: '',
    };
// Метод isHighlighted в компоненте Struct
    isHighlighted = (text) => {
        return this.props.searchTerm && text.toLowerCase().includes(this.props.searchTerm.toLowerCase());
    }
    componentDidUpdate(prevProps) {
        if (
            JSON.stringify(prevProps.fields) !== JSON.stringify(this.props.fields) ||
            JSON.stringify(prevProps.methods) !== JSON.stringify(this.props.methods) ||
            prevProps.name !== this.props.name
        ) {
            this.setState({
                name: this.props.name,
                fields: this.props.fields,
                methods: this.props.methods
            });
        }
    }

    highlightText = (text) => {
        const { searchTerm } = this.props;
        if (!searchTerm) return text;

        const regex = new RegExp(`(${searchTerm})`, 'gi');
        return text.replace(regex, '<span class="highlight">$1</span>');
    };

    getInput = (options) => {
        let { name, ref, value, onChange, onBlur } = options;
        value = value || '';
        onChange = onChange || noop;
        onBlur = onBlur || noop;

        const isHighlighted = this.isHighlighted(value);

        return (
            <AutosizeInput
                className={`${name} ${isHighlighted ? 'highlighted' : ''}`}
                autoComplete='off'
                name={name}
                ref={ref}
                value={value}
                minWidth={STRUCT_MIN_WIDTH}
                onBlur={onBlur}
                onChange={onChange}
                onKeyPress={(e) => {
                    if (e.which === 13) {
                        this.headerRef.current.blur();
                    }
                }}
            />
        );
    };

    render() {
        let fields = this.state.fields.map((field, i) => {
            const TYPES = ['string', 'int', 'bool'];
            let typeClass = TYPES.indexOf(field.type.struct) !== -1 ? field.type.struct : 'other';
            return (
                <li key={i} className={`field ${this.isHighlighted(field.name) || this.isHighlighted(field.type.literal) ? 'highlighted' : ''}`}>
                <span className='left'>
                    <span className='field icon' onClick={() => this.onRemoveField(i)}>
                        <span className='f'>f</span>
                        <span className='x'>x</span>
                    </span>
                    {this.getInput({
                        name: 'name',
                        ref: (ref) => this.fieldNameRefs[i] = ref,
                        value: field.name,
                        onChange: (e) => this.onFieldNameChange(i, e),
                        onBlur: (e) => this.onFieldNameBlur(i, e),
                    })}
                </span>
                    <span className='right'>
                    {this.getInput({
                        name: ['type', typeClass].join(' '),
                        ref: (ref) => this.fieldTypeRefs[i] = ref,
                        value: field.type.literal,
                        onChange: (e) => this.onFieldTypeChange(i, e),
                        onBlur: (e) => this.onFieldTypeBlur(i, e),
                    })}
                </span>
                </li>
            );
        });

        let methods = this.state.methods.map((method, i) => {
            return (
                <li key={i} className={`method ${
                    this.isHighlighted(method.name) ||
                    (method.returnType && method.returnType.some(type => this.isHighlighted(type.literal)))
                        ? 'highlighted'
                        : ''
                }`}>
                <span className='left'>
                    <span className='method icon'>m</span>
                    {this.getInput({
                        name: 'name',
                        ref: (ref) => this.methodNameRefs[i] = ref,
                        value: method.name,
                        onChange: (e) => this.onMethodNameChange(i, e),
                        onBlur: (e) => this.onMethodNameBlur(i, e),
                    })}
                </span>
                    <span className='right'>
                    {(method.returnType || []).map((type, j) => (
                        <span key={j} className='return-type'>
                            {this.getInput({
                                name: 'type',
                                ref: (ref) => this.methodReturnRefs[i] = ref,
                                value: type.literal,
                                onChange: (e) => this.onMethodReturnTypeChange(i, j, e),
                                onBlur: (e) => this.onMethodReturnTypeBlur(i, j, e),
                            })}
                        </span>
                    ))}
                </span>
                </li>
            );
        });

        return (
            <div className={`Struct ${this.props.className} ${this.isHighlighted(this.state.name) ? 'highlighted' : ''}`}>
                <header className='header'>
                <span className='class icon' onClick={this.onAddField}>
                    <span className='c'>c</span>
                    <span className='p'>+</span>
                </span>
                    {this.getInput({
                        name: 'name',
                        ref: this.headerRef,
                        value: this.state.name,
                        onChange: this.onNameChange,
                        onBlur: this.onNameBlur,
                    })}
                    <span className='delete icon' onClick={this.onDelete}>x</span>
                </header>
                <ol className='fields'>
                    {fields}
                </ol>
                <ol className='methods'>
                    {methods}
                </ol>
            </div>
        );
    }
    onFieldTypeChange = (key, e) => {
        let newFields = this.state.fields.slice(0);
        newFields[key].type.literal = e.target.value;
        this.setState({
            fields: newFields,
        });
    }

    onFieldTypeBlur = (key, e) => {
        this.props.onFieldTypeChange({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
            key,
            newFieldType: e.target.value,
        });
    }

    onFieldNameChange = (key, e) => {
        let newFields = this.state.fields.slice(0);
        newFields[key].name = e.target.value;
        this.setState({
            fields: newFields,
        });
    }

    onFieldNameBlur = (key, e) => {
        this.props.onFieldNameChange({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
            key,
            newFieldName: e.target.value,
        });
    }

    onRemoveField = (key) => {
        this.props.onRemoveField({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
            key,
        });
    }

    onAddField = () => {
        this.headerRef.current.blur();
        this.props.onAddField({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
        });
    }

    onHeaderClick = () => {
        this.headerRef.current.select();
    }

    onDelete = () => {
        this.props.onDelete({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
        });
    }

    onNameChange = (e) => {
        this.setState({
            name: e.target.value,
        });
    }

    onMethodNameChange = (index, e) => {
        const newMethods = [...this.state.methods];
        newMethods[index].name = e.target.value;
        this.setState({methods: newMethods});
    }

    onMethodNameBlur = (index, e) => {
        this.props.onMethodNameChange({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
            methodIndex: index,
            newMethodName: e.target.value,
        });
    }

    onMethodReturnTypeChange = (methodIndex, typeIndex, e) => {
        const newMethods = [...this.state.methods];
        newMethods[methodIndex].returnType[typeIndex].literal = e.target.value;
        this.setState({methods: newMethods});
    }

    onNameBlur = (e) => {
        this.props.onNameChange({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
            newName: e.target.value,
        });
    }

    onMethodReturnTypeBlur = (methodIndex, typeIndex, e) => {
        this.props.onMethodReturnTypeChange({
            package: this.props.package,
            file: this.props.file,
            name: this.props.name,
            methodIndex: methodIndex,
            typeIndex: typeIndex,
            newReturnType: e.target.value,
        });
    }
}

export default Struct;