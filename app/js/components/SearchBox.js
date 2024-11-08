import React, { Component } from 'react';

class SearchBox extends Component {
    constructor(props) {
        super(props);
        this.state = {
            query: this.props.value || '',
        };
    }

    static defaultProps = {
        className: '',
        value: '',
        placeholder: 'search...',
        onSearch: () => {}
    };

    render() {
        return (
            <div className={`SearchBox ${this.props.className}`}>
                <input
                    type='text'
                    value={this.state.query}
                    className='input'
                    placeholder={this.props.placeholder}
                    onChange={this.changeHandler}
                    onKeyPress={this.handleKeyPress}
                />
            </div>
        );
    }

    changeHandler = (e) => {
        this.setState({ query: e.target.value });
    }

    handleKeyPress = (e) => {
        if (e.key === 'Enter') {
            this.props.onSearch(this.state.query);
        }
    }
}

export default SearchBox;