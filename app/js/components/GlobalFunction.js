import React, { useState } from 'react';

const GlobalFunction = ({ functions = [], isHighlighted }) => {
    const [expandedPackages, setExpandedPackages] = useState({});

    const functionsByPackage = functions.reduce((acc, func) => {
        acc[func.package] = acc[func.package] || [];
        acc[func.package].push(func);
        return acc;
    }, {});

    const togglePackage = (packageName) => {
        setExpandedPackages((prev) => ({
            ...prev,
            [packageName]: !prev[packageName],
        }));
    };

    return (
        <div className="global-functions-container">
            <h2>Global Functions</h2>
            {Object.entries(functionsByPackage).map(([packageName, packageFunctions]) => (
                <div key={packageName} className="package-functions">
                    <h3 onClick={() => togglePackage(packageName)} style={{ cursor: 'pointer' }}>
                        {packageName}
                        <span className="toggle-icon">{expandedPackages[packageName] ? '▼' : '▶'}</span>
                    </h3>
                    {expandedPackages[packageName] && packageFunctions.map((func, index) => (
                        <div
                            key={`${func.package}-${func.file}-${func.name}-${index}`}
                            className={`GlobalFunction ${isHighlighted(func.name) ? 'highlighted' : ''}`}
                        >
                            <h4>{func.name || 'Unnamed Function'}</h4>
                            <p className="file-name">File: {func.file || 'Unknown File'}</p>
                            <p className="parameters">
                                Parameters:
                                {func.parameters?.length > 0 ? (
                                    func.parameters.map(({ name, type }) => (
                                        <span key={name} className="parameter">
                                            {name}: <span className="type">{type?.literal || 'unknown'}</span>
                                        </span>
                                    ))
                                ) : (
                                    <span className="no-params">No parameters</span>
                                )}
                            </p>
                            <p className="return-type">
                                Return:
                                {func.returnType?.length > 0 ? (
                                    func.returnType.map((t, i) => (
                                        <span key={i} className="type">{t?.literal || 'unknown'}</span>
                                    ))
                                ) : (
                                    <span className="no-return">No return value</span>
                                )}
                            </p>
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
};

export default GlobalFunction;
