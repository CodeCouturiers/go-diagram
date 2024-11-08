import React, { Component, createRef } from 'react';
import Struct from './Struct';
import Button from './Button';
import SearchBox from './SearchBox';
import GlobalFunction from './GlobalFunction';

const createValidSelector = (str) => {
  return str.replace(/[^\w\s-]/g, '-');
};

class UMLDiagram extends Component {
  constructor(props) {
    super(props);
    this.state = {
      dragging: false,
      dragOrigin: null,
      clickStart: null,
      selection: {
        pkg: null,
        file: null,
        struct: null,
      },
      position: {
        x: 0,
        y: 0,
      },
      searchTerm: '',
      isTransitioning: false,
    };
    this.diagramRef = createRef();
  }

  static defaultProps = {
    actions: {},
    data: {
      packages: [],
      edges: [],
      globalFunctions: []
    },
    miniMap: false,
  };

  componentDidUpdate(prevProps) {
    if (JSON.stringify(prevProps.data) !== JSON.stringify(this.props.data)) {
      this.setState({ isTransitioning: true }, () => {
        setTimeout(() => {
          this.setState({ isTransitioning: false });
        }, 300);
      });
    }
  }

  handleSearch = (query) => {
    this.setState({ searchTerm: query.toLowerCase() });
  }

  isHighlighted = (text) => {
    const { searchTerm } = this.state;
    return searchTerm && text.toLowerCase().includes(searchTerm);
  }

  render() {
    const { data, miniMap } = this.props;
    const { isTransitioning, position, dragging, searchTerm } = this.state;

    if (!data || !data.packages) {
      return <div>Loading...</div>;
    }

    const transformList = [`translate(${position.x}px, ${position.y}px)`];
    if (miniMap) {
      transformList.push('scale(0.3)');
    }
    const transform = {
      transform: transformList.join(' ')
    };

    const packages = this.renderPackages();
    const globalFunctions = this.renderGlobalFunctions();
    const edges = this.renderEdges();

    return (
        <div
            className={`UMLDiagram ${dragging ? 'dragging' : ''} ${isTransitioning ? 'transitioning' : ''}`}
            onMouseDown={this.onMouseDown}
            onMouseUp={this.onMouseUp}
            onMouseLeave={this.onMouseLeave}
            onMouseMove={this.onMouseMove}
            onTouchStart={this.onMouseDown}
            onTouchEnd={this.onMouseUp}
            onTouchMove={this.onMouseMove}
        >
          <SearchBox
              onSearch={this.handleSearch}
              className="uml-search"
              placeholder="Search structs, fields, methods, functions..."
          />
          <div className="diagram" style={transform} ref={this.diagramRef}>
            <section className="packages">{packages}</section>
            <section className="global-functions">{globalFunctions}</section>
            <section className="edges">{edges}</section>
          </div>
        </div>
    );
  }

  renderPackages() {
    const { data } = this.props;
    const { selection } = this.state;

    return data.packages.map((pkg) => (
        <section
            key={pkg.name}
            className={`package ${(selection.pkg === pkg.name) ? 'selected' : ''} ${this.isHighlighted(pkg.name) ? 'highlighted' : ''}`}
            onClick={(e) => this.onPackageClick(pkg, e)}
        >
          <h3 className="title">{pkg.name}</h3>
          {pkg.files.map((file) => this.renderFile(pkg, file))}
        </section>
    ));
  }

  renderFile(pkg, file) {
    const { selection } = this.state;

    return (
        <div
            key={file.name}
            className={`file ${(selection.file === file.name) ? 'selected' : ''} ${this.isHighlighted(file.name) ? 'highlighted' : ''}`}
            onClick={(e) => this.onFileClick({ pkg, file }, e)}
        >
          <h3 className="title">{file.name}</h3>
          <Button
              className="addStruct"
              value="+"
              onClick={() => this.addStruct({ package: pkg.name, file: file.name })}
          />
          {file.structs.map((struct) => this.renderStruct(pkg, file, struct))}
        </div>
    );
  }

  renderStruct(pkg, file, struct) {
    return (
        <Struct
            key={`${pkg.name}-${file.name}-${struct.name}`}
            className={`${this.getStructRef(pkg, file, struct)} ${this.isHighlighted(struct.name) ? 'highlighted' : ''}`}
            package={pkg.name}
            file={file.name}
            onDelete={this.props.actions.deleteStruct}
            onNameChange={this.props.actions.changeStructName}
            onFieldTypeChange={this.props.actions.changeStructFieldType}
            onFieldNameChange={this.props.actions.changeStructFieldName}
            onAddField={this.props.actions.addStructField}
            onRemoveField={this.props.actions.removeStructField}
            name={struct.name}
            fields={struct.fields || []}
            methods={struct.methods || []}
            onMethodNameChange={this.props.actions.changeStructMethodName}
            onMethodReturnTypeChange={this.props.actions.changeStructMethodReturnType}
            onAddMethod={this.props.actions.addStructMethod}
            onRemoveMethod={this.props.actions.removeStructMethod}
            searchTerm={this.state.searchTerm}
        />
    );
  }

  renderGlobalFunctions() {
    const { data } = this.props;

    return (
        <GlobalFunction
            functions={data.globalFunctions}
            isHighlighted={this.isHighlighted}
        />
    );
  }

  renderEdges() {
    const { data } = this.props;

    return (
        <svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%">
          <defs>
            <marker id="arrowhead" markerWidth="10" markerHeight="7"
                    refX="0" refY="3.5" orient="auto">
              <polygon points="0 0, 10 3.5, 0 7" />
            </marker>
          </defs>
          {data.edges.map((edge, index) => this.renderEdge(edge, index))}
        </svg>
    );
  }

  renderEdge(edge, index) {
    if (!edge || !edge.from || !edge.to) {
      console.warn('Invalid edge data', edge);
      return null;
    }

    const start = this.getNodePosition(edge.from);
    const end = this.getNodePosition(edge.to);

    if (!start || !end) {
      return null;
    }

    const path = `M${start.x},${start.y} C${(start.x + end.x) / 2},${start.y} ${(start.x + end.x) / 2},${end.y} ${end.x},${end.y}`;

    return (
        <path
            key={index}
            d={path}
            fill="none"
            stroke="black"
            strokeWidth="2"
            markerEnd="url(#arrowhead)"
        />
    );
  }

  getNodePosition(node) {
    if (!node || !node.packageName || !node.fileName || !node.structName) {
      console.warn('Invalid node data', node);
      return null;
    }
    const selector = this.getStructRef(node.packageName, node.fileName, node.structName);
    if (!selector) return null;

    const element = document.querySelector(`.${selector}`);
    if (!element) {
      console.warn(`Element not found for selector: ${selector}`);
      return null;
    }

    const rect = element.getBoundingClientRect();
    return {
      x: rect.left + rect.width / 2,
      y: rect.top + rect.height / 2
    };
  }

  getStructRef = (pkg, file, struct) => {
    if (!pkg || !file || !struct) {
      console.warn('Invalid data passed to getStructRef', { pkg, file, struct });
      return '';
    }
    const pkgName = typeof pkg === 'object' ? pkg.name : pkg;
    const fileName = typeof file === 'object' ? file.name : file;
    const structName = typeof struct === 'object' ? struct.name : struct;

    if (!pkgName || !fileName || !structName) {
      console.warn('Missing name in getStructRef', { pkgName, fileName, structName });
      return '';
    }

    return createValidSelector(`${pkgName}-${fileName}-${structName}`);
  }

  onMouseDown = (e) => {
    const { pageX, pageY } = e.touches ? e.touches[0] : e;
    const { x, y } = this.state.position;
    this.setState({
      dragging: true,
      dragOrigin: { x: pageX - x, y: pageY - y }
    });
  }

  onMouseUp = () => {
    this.setState({ dragging: false });
  }

  onMouseLeave = () => {
    this.setState({ dragging: false });
  }

  onMouseMove = (e) => {
    if (this.state.dragging) {
      const { pageX, pageY } = e.touches ? e.touches[0] : e;
      const { dragOrigin } = this.state;
      this.setState({
        position: {
          x: pageX - dragOrigin.x,
          y: pageY - dragOrigin.y,
        },
      });
    }
  }

  onPackageClick = (pkg, e) => {
    e.stopPropagation();
    this.setState({
      selection: {
        pkg: pkg.name,
        file: null,
        struct: null,
      }
    });
  }

  onFileClick = ({ pkg, file }, e) => {
    e.stopPropagation();
    this.setState({
      selection: {
        pkg: pkg.name,
        file: file.name,
        struct: null,
      }
    });
  }

  addStruct = (file) => {
    this.props.actions.onAddStruct(file);
  }
}

export default UMLDiagram;