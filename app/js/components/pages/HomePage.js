import React, { Component } from 'react';
import { connect } from 'react-redux';
import { bindActionCreators } from 'redux';
import UMLDiagram from '../UMLDiagram';
import * as AppActions from '../../actions/AppActions';
import Connection from '../../utils/Connection';

class HomePage extends Component {
  constructor(props) {
    super(props);
    this.state = {
      packageData: null,
    };
    this.setUpConnection();
  }

  setUpConnection() {
    Connection.setUp();
    Connection.onMessage((data) => {
      console.log('WebSocket message received:', data);
      if (data.error) {
        console.error('WebSocket error:', data.error);
        alert(data.error);
      } else if (data.fileChanged || data.packages) {
        console.log('Updating package data:', data);
        this.setState({ packageData: data });
        this.props.actions.updateFromFileChange(data);
      }
    });
  }

  componentDidUpdate(prevProps, prevState) {
    console.log('HomePage did update');
    console.log('Previous props:', prevProps);
    console.log('Current props:', this.props);
    console.log('Previous state:', prevState);
    console.log('Current state:', this.state);
  }

  render() {
    console.log('HomePage render');
    console.log('Current state:', this.state);
    console.log('Current props:', this.props);

    const { packageData } = this.state;

    if (!packageData || packageData.packages.length === 0) {
      console.log('Loading...');
      return <div>Loading...</div>;
    }

    return (
        <div className="HomePage">
          <UMLDiagram
              actions={this.props.actions}
              data={packageData}
          />
        </div>
    );
  }
}

function mapStateToProps(state) {
  console.log('mapStateToProps:', state);
  return {
    data: state
  };
}

function mapDispatchToProps(dispatch) {
  return {
    actions: bindActionCreators(AppActions, dispatch)
  };
}

export default connect(mapStateToProps, mapDispatchToProps)(HomePage);