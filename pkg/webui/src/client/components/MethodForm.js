// MethodForm component - generates dynamic forms from method parameters

import m from 'mithril'
import TypeInput from './TypeInput.js';
import { resolveType, findStruct } from '../utils/types.js';

export default {
    formValues: {},
    
    oninit(vnode) {
        // Initialize form values with defaults
        const { method } = vnode.attrs;
        if (method) {
            this.lastMethodName = method.name;
        }
        this.initializeForm(vnode);
    },
    
    onupdate(vnode) {
        // Re-initialize if method changed
        const currentMethod = vnode.attrs.method;
        if (currentMethod && currentMethod.name !== this.lastMethodName) {
            this.initializeForm(vnode);
            this.lastMethodName = currentMethod.name;
        }
    },
    
    initializeForm(vnode) {
        const { method } = vnode.attrs;
        if (!method || !method.parameters) {
            // Reuse existing object, just clear it
            if (!this.formValues) {
                this.formValues = {};
            } else {
                Object.keys(this.formValues).forEach(key => delete this.formValues[key]);
            }
            return;
        }
        
        // Initialize with null/undefined - let TypeInput show placeholders
        // Reuse existing formValues object instead of creating a new one
        // This ensures that references to formValues remain valid
        if (!this.formValues) {
            this.formValues = {};
        }
        // Clear existing values and set new ones
        Object.keys(this.formValues).forEach(key => {
            if (!method.parameters.find(p => p.name === key)) {
                delete this.formValues[key];
            }
        });
        method.parameters.forEach(param => {
            const resolved = resolveType(param.type);
            if (resolved.kind === 'userDefined' && vnode.attrs.typeRegistry && findStruct(resolved.name, vnode.attrs.typeRegistry)) {
                this.formValues[param.name] = {};
            } else {
                this.formValues[param.name] = null;
            }
        });
        
        if (vnode.attrs.onFormChange) {
            vnode.attrs.onFormChange(this.formValues);
        }
    },
    
    view(vnode) {
        const { method, typeRegistry, onSubmit } = vnode.attrs;
        
        if (!method) return null;
        
        return m('div.card.mb-3', [
            m('div.card-header', [
                m('h5.mb-0', method.name),
                method.comment && m('div.small.text-muted.mt-1', method.comment)
            ]),
            m('div.card-body', [
                method.parameters && method.parameters.length > 0 ? [
                    method.parameters.map(param =>
                        m('div.mb-3', [
                            m('label.form-label', [
                                m('strong', param.name),
                                m('span.text-muted.ml-2', ' - ' + this.formatType(param.type)),
                                param.comment && m('div.small.text-muted.mt-1', param.comment)
                            ]),
                            m(TypeInput, {
                                type: param.type,
                                value: this.formValues[param.name],
                                onchange: (newValue) => {
                                    this.formValues[param.name] = newValue;
                                    if (vnode.attrs.onFormChange) {
                                        vnode.attrs.onFormChange(this.formValues);
                                    }
                                },
                                registry: typeRegistry,
                                path: param.name
                            })
                        ])
                    ),
                    m('div.mt-4', [
                        m('button.btn.btn-primary', {
                            onclick: () => {
                                if (onSubmit) {
                                    onSubmit(this.formValues);
                                }
                            }
                        }, 'Call Method')
                    ])
                ] : [
                    m('p.text-muted', 'This method has no parameters.'),
                    m('button.btn.btn-primary', {
                        onclick: () => {
                            if (onSubmit) {
                                onSubmit({});
                            }
                        }
                    }, 'Call Method')
                ]
            ])
        ]);
    },
    
    formatType(type) {
        if (!type) return 'void';
        if (type.builtIn) return type.builtIn;
        if (type.array) return '[]' + this.formatType(type.array);
        if (type.mapValue) return 'map[string]' + this.formatType(type.mapValue);
        if (type.userDefined) return type.userDefined;
        return 'unknown';
    },
    
    getDefaultValue(typeDef) {
        if (!typeDef) return null;
        
        if (typeDef.builtIn) {
            switch (typeDef.builtIn) {
                case 'string': return '';
                case 'int': return 0;
                case 'float': return 0.0;
                case 'bool': return false;
                default: return null;
            }
        }
        
        if (typeDef.array) {
            return [];
        }
        
        if (typeDef.mapValue) {
            return {};
        }
        
        if (typeDef.userDefined) {
            // For structs, we'd need to initialize with field defaults
            // For now, return null and let TypeInput handle it
            return null;
        }
        
        return null;
    }
};

